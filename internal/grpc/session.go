package grpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/openkcm/common-sdk/pkg/openid"
	"github.com/patrickmn/go-cache"

	sessionv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/session/v1"
	typesv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/session"
)

type SessionServerOption func(*SessionServer)

func WithQueryParametersIntrospect(params []string) SessionServerOption {
	return func(s *SessionServer) {
		s.queryParametersIntrospect = params
	}
}

type SessionServer struct {
	sessionv1.UnimplementedServiceServer

	sessionRepo  session.Repository
	providerRepo oidc.ProviderRepository
	httpClient   *http.Client

	queryParametersIntrospect []string
	idleSessionTimeout        time.Duration

	cache *cache.Cache
}

func NewSessionServer(
	sessionRepo session.Repository,
	providerRepo oidc.ProviderRepository,
	httpClient *http.Client,
	idleSessionTimeout time.Duration,
	opts ...SessionServerOption,
) *SessionServer {
	s := &SessionServer{
		sessionRepo:        sessionRepo,
		providerRepo:       providerRepo,
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
	slogctx.Debug(ctx, "GetSession called")
	defer slogctx.Debug(ctx, "GetSession completed")

	active, err := s.sessionRepo.IsActive(ctx, req.GetSessionId())
	if err != nil {
		slogctx.Error(ctx, "failed to get the session active state", "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	if !active {
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Load session for the given session ID
	sess, err := s.sessionRepo.LoadSession(ctx, req.GetSessionId())
	if err != nil {
		slogctx.Warn(ctx, "Is this an attack? Could not load session", "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Get the stored trust for the given tenant ID
	trust, err := s.providerRepo.Get(ctx, req.GetTenantId())
	if err != nil {
		slogctx.Warn(ctx, "Is this an attack? Could not get OIDC provider", "issuer", sess.Issuer, "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}
	if trust.Blocked {
		slogctx.Warn(ctx, "OIDC provider is blocked", "issuer", sess.Issuer)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Compare fingerprints
	if sess.Fingerprint != req.GetFingerprint() {
		slogctx.Warn(ctx, "Is this an attack? Fingerprints do not match", "session_fingerprint", sess.Fingerprint, "request_fingerprint", req.GetFingerprint())
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Compare tenant IDs
	if sess.TenantID != req.GetTenantId() {
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
	result, err := s.introspectToken(ctx, sess.AccessToken, &trust)
	if err != nil {
		slogctx.Error(ctx, "Could not introspect access token", "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, err
	}
	if !result.Active {
		slogctx.Warn(ctx, "Access token is not active", "result", result)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}
	if result.Groups != nil {
		response.Groups = result.Groups
	}

	// Bump the session to keep it active
	if err := s.sessionRepo.BumpActive(ctx, req.GetSessionId(), s.idleSessionTimeout); err != nil {
		slogctx.Error(ctx, "failed to bump the session status", "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Return info of the valid session
	return response, nil
}

func (s *SessionServer) GetOIDCProvider(ctx context.Context, req *sessionv1.GetOIDCProviderRequest) (*sessionv1.GetOIDCProviderResponse, error) {
	provider, err := s.providerRepo.Get(ctx, req.GetTenantId())
	if err != nil {
		return nil, fmt.Errorf("getting odic provider: %w", err)
	}

	return &sessionv1.GetOIDCProviderResponse{
		Provider: &typesv1.OIDCProvider{
			IssuerUrl: provider.IssuerURL,
			JwksUri:   provider.JWKSURI,
			Audiences: provider.Audiences,
		},
	}, nil
}

func (s *SessionServer) introspectToken(ctx context.Context, token string, trust *oidc.Provider) (openid.IntrospectResponse, error) {
	const introspectPrefix = "introspect_"

	// first check the cache for a recent introspection result for this token
	cacheKey := introspectPrefix + token
	cache, ok := s.cache.Get(cacheKey)
	if ok {
		//nolint:forcetypeassert
		return cache.(openid.IntrospectResponse), nil
	}

	// create the provider for the given issuer
	provider, err := openid.NewProvider(trust.IssuerURL, trust.Audiences,
		openid.WithIntrospectQueryParameters(trust.GetIntrospectParameters(s.queryParametersIntrospect)),
	)
	if err != nil {
		slogctx.Error(ctx, "Could not create OpenID provider", "issuer", trust.IssuerURL, "error", err)
		return openid.IntrospectResponse{Active: false}, err
	}

	// introspect the token and cache the result
	intr, err := provider.IntrospectToken(ctx, token)
	if err != nil {
		if errors.Is(err, openid.ErrNoIntrospectionEndpoint) {
			slogctx.Debug(ctx, "No introspection endpoint configured", "issuer", provider.Issuer)
			return openid.IntrospectResponse{Active: true}, nil
		}
		slogctx.Error(ctx, "Could not introspect access token", "error", err)
		return openid.IntrospectResponse{Active: false}, err
	}
	s.cache.Set(cacheKey, intr, 0)

	return intr, nil
}
