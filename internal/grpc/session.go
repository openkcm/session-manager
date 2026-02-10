package grpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/openkcm/common-sdk/pkg/oidc"
	"github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	sessionv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/session/v1"
	typesv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/session"
	"github.com/openkcm/session-manager/internal/trust"
)

type SessionServerOption func(*SessionServer)

func WithQueryParametersIntrospect(params []string) SessionServerOption {
	return func(s *SessionServer) {
		s.queryParametersIntrospect = params
	}
}

func WithAllowHttpScheme(allow bool) SessionServerOption {
	return func(s *SessionServer) {
		s.allowHttpScheme = allow
	}
}

type SessionServer struct {
	sessionv1.UnimplementedServiceServer

	sessionRepo session.Repository
	trustRepo   trust.OIDCMappingRepository
	httpClient  *http.Client

	queryParametersIntrospect []string
	idleSessionTimeout        time.Duration
	allowHttpScheme           bool

	cache *cache.Cache
}

func NewSessionServer(
	sessionRepo session.Repository,
	trustRepo trust.OIDCMappingRepository,
	httpClient *http.Client,
	idleSessionTimeout time.Duration,
	opts ...SessionServerOption,
) *SessionServer {
	s := &SessionServer{
		sessionRepo:        sessionRepo,
		trustRepo:          trustRepo,
		httpClient:         httpClient,
		idleSessionTimeout: idleSessionTimeout,
		cache:              cache.New(2*time.Minute, 10*time.Minute),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

func (s *SessionServer) GetSession(ctx context.Context, req *sessionv1.GetSessionRequest) (*sessionv1.GetSessionResponse, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "get_session")
	defer span.End()

	slogctx.Debug(ctx, "GetSession called")
	defer slogctx.Debug(ctx, "GetSession completed")

	active, err := s.sessionRepo.IsActive(ctx, req.GetSessionId())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to check session state")
		slogctx.Error(ctx, "failed to get the session active state", "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	if !active {
		span.SetStatus(codes.Ok, "")
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Load session for the given session ID
	sess, err := s.sessionRepo.LoadSession(ctx, req.GetSessionId())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to load a session")
		slogctx.Warn(ctx, "Is this an attack? Could not load session", "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Get trust mapping for the given tenant ID
	mapping, err := s.trustRepo.Get(ctx, req.GetTenantId())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get an oidc mapping")
		slogctx.Warn(ctx, "Is this an attack? Could not get trust mapping", "issuer", sess.Issuer, "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}
	if mapping.Blocked {
		span.SetStatus(codes.Ok, "the tenant is blocked")
		slogctx.Warn(ctx, "Tenant is blocked", "issuer", sess.Issuer)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Compare fingerprints
	if sess.Fingerprint != req.GetFingerprint() {
		span.SetStatus(codes.Ok, "fingerprint mismatch")
		slogctx.Warn(ctx, "Is this an attack? Fingerprints do not match", "session_fingerprint", sess.Fingerprint, "request_fingerprint", req.GetFingerprint())
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Compare tenant IDs
	if sess.TenantID != req.GetTenantId() {
		span.SetStatus(codes.Ok, "tenant id mismatch")
		slogctx.Warn(ctx, "Is this an attack? Tenant IDs do not match", "session_tenant_id", sess.TenantID, "request_tenant_id", req.GetTenantId())
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	response := &sessionv1.GetSessionResponse{
		Valid:       true,
		Issuer:      sess.Issuer,
		Subject:     sess.Claims.Subject,
		GivenName:   sess.Claims.GivenName,
		FamilyName:  sess.Claims.FamilyName,
		Email:       sess.Claims.Email,
		Groups:      sess.Claims.Groups,
		AuthContext: sess.AuthContext,
	}

	// Introspect access token
	result, err := s.introspectToken(ctx, sess.AccessToken, &mapping)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to introspect an access token")
		slogctx.Error(ctx, "Could not introspect access token", "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, err
	}
	if !result.Active {
		slogctx.Warn(ctx, "Access token is not active", "result", result)
		span.SetStatus(codes.Ok, "access token is not active")
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	if result.Groups != nil {
		response.Groups = result.Groups
	}

	// Bump the session to keep it active
	if err := s.sessionRepo.BumpActive(ctx, req.GetSessionId(), s.idleSessionTimeout); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to bump the session status")
		slogctx.Error(ctx, "failed to bump the session status", "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Return info of the valid session
	span.SetStatus(codes.Ok, "")
	return response, nil
}

func (s *SessionServer) GetOIDCProvider(ctx context.Context, req *sessionv1.GetOIDCProviderRequest) (*sessionv1.GetOIDCProviderResponse, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "get_oidc_provider")
	defer span.End()

	provider, err := s.trustRepo.Get(ctx, req.GetTenantId())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get an oidc provider")
		return nil, fmt.Errorf("getting odic provider: %w", err)
	}

	span.SetStatus(codes.Ok, "")
	return &sessionv1.GetOIDCProviderResponse{
		Provider: &typesv1.OIDCProvider{
			IssuerUrl: provider.IssuerURL,
			JwksUri:   provider.JWKSURI,
			Audiences: provider.Audiences,
		},
	}, nil
}

func (s *SessionServer) introspectToken(ctx context.Context, token string, oidcTrust *trust.OIDCMapping) (oidc.Introspection, error) {
	const introspectPrefix = "introspect_"

	// first check the cache for a recent introspection result for this token
	cacheKey := introspectPrefix + token
	cache, ok := s.cache.Get(cacheKey)
	if ok {
		//nolint:forcetypeassert
		return cache.(oidc.Introspection), nil
	}

	// create the provider for the given issuer
	provider, err := oidc.NewProvider(oidcTrust.IssuerURL, oidcTrust.Audiences,
		oidc.WithIntrospectQueryParameters(oidcTrust.GetIntrospectParameters(s.queryParametersIntrospect)),
		oidc.WithAllowHttpScheme(s.allowHttpScheme),
	)
	if err != nil {
		slogctx.Error(ctx, "Could not create OpenID provider", "issuer", oidcTrust.IssuerURL, "error", err)
		return oidc.Introspection{Active: false}, err
	}

	// introspect the token and cache the result
	intr, err := provider.IntrospectToken(ctx, token)
	if err != nil {
		if errors.Is(err, oidc.ErrNoIntrospectionEndpoint) {
			slogctx.Debug(ctx, "No introspection endpoint configured", "issuer", provider.Issuer)
			return oidc.Introspection{Active: true}, nil
		}
		slogctx.Error(ctx, "Could not introspect access token", "error", err)
		return oidc.Introspection{Active: false}, err
	}
	s.cache.Set(cacheKey, intr, 0)

	return intr, nil
}
