package session

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/openkcm/common-sdk/pkg/oidc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc/status"

	rpcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/rpc/v1"
	sessionv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/session/v1"
	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	typesv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"
	slogctx "github.com/veqryn/slog-context"
	grpccodes "google.golang.org/grpc/codes"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/credentials"
	"github.com/openkcm/session-manager/internal/debugtools"
	internalsession "github.com/openkcm/session-manager/internal/session"
)

const defaultIntrospectionCacheExpiration = 30 * time.Second

var debugSettingSMDumpTransport = debugtools.NewSetting("smdumptransport")

type Server struct {
	sessionv1.UnimplementedServiceServer

	sessionRepo internalsession.Repository
	trust       sessionmanager.Trust
	newCreds    credentials.Builder

	queryParametersIntrospect []string
	idleSessionTimeout        time.Duration
	allowHttpScheme           bool
	clientID                  string

	// cache introspection results
	introspectionCache *ttlcache.Cache[string, oidc.Introspection]
}

func NewServer(
	ctx context.Context,
	sessionRepo internalsession.Repository,
	trust sessionmanager.Trust,
	idleSessionTimeout time.Duration,
	clientID string,
	opts ...Option,
) *Server {
	s := &Server{
		sessionRepo:        sessionRepo,
		trust:              trust,
		idleSessionTimeout: idleSessionTimeout,
		newCreds:           func(clientID string) credentials.TransportCredentials { return credentials.NewInsecure(clientID) },
		clientID:           clientID,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	s.introspectionCache = ttlcache.New(ttlcache.WithTTL[string, oidc.Introspection](defaultIntrospectionCacheExpiration))
	go s.introspectionCache.Start()
	go func(ctx context.Context) {
		<-ctx.Done()
		s.introspectionCache.Stop()
	}(ctx)

	return s
}

func (s *Server) GetSession(ctx context.Context, req *sessionv1.GetSessionRequest) (*sessionv1.GetSessionResponse, error) {
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

	// Get trust for the given tenant ID
	trust, err := s.trust.Get(ctx, req.GetTenantId())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get trust")
		slogctx.Warn(ctx, "Is this an attack? Could not get trust", "issuer", sess.Issuer, "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}
	if trust.GetBlocked() {
		slogctx.Warn(ctx, "Tenant is blocked", "issuer", sess.Issuer)
		span.SetStatus(codes.Ok, "the tenant is blocked")
		st := status.New(grpccodes.FailedPrecondition, "the tenant is blocked")
		dt, err := st.WithDetails(&rpcv1.PreconditionFailure{
			Violations: []*rpcv1.PreconditionFailure_Violation{
				{
					Type:        violationTenantBlocked,
					Subject:     "tenant:" + req.GetTenantId(),
					Description: "The tenant is blocked",
				},
			},
		})
		if err != nil {
			slogctx.Error(ctx, "Failed to add error details", "error", err)
			return nil, st.Err()
		}

		return nil, dt.Err()
	}

	// Compare tenant IDs
	if sess.TenantID != req.GetTenantId() {
		span.SetStatus(codes.Ok, "tenant id mismatch")
		slogctx.Warn(ctx, "Is this an attack? Tenant IDs do not match", "sessionTenantId", sess.TenantID, "requestTenantId", req.GetTenantId())
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
	result, err := s.introspectToken(ctx, sess.AccessToken, trust.GetOidc())
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

// GetOIDCProvider implements a compatibility level with the OIDC API.
// Deprecated: use GetTrust instead.
// TODO: remove this method once the lifecycle of deprecated and compatibility layers is reached to the end.
//
//nolint:staticcheck
func (s *Server) GetOIDCProvider(ctx context.Context, req *sessionv1.GetOIDCProviderRequest) (*sessionv1.GetOIDCProviderResponse, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "get_oidc_provider")
	defer span.End()

	provider, err := s.trust.Get(ctx, req.GetTenantId())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get an oidc provider")
		return nil, fmt.Errorf("getting odic provider: %w", err)
	}

	oidc := provider.GetOidc()

	span.SetStatus(codes.Ok, "")
	return &sessionv1.GetOIDCProviderResponse{
		Provider: &typesv1.OIDCProvider{
			IssuerUrl: oidc.GetIssuer(),
			JwksUri:   oidc.GetJwksUri(),
			Audiences: oidc.GetAudiences(),
		},
	}, nil
}

func (s *Server) GetTrust(ctx context.Context, req *sessionv1.GetTrustRequest) (*sessionv1.GetTrustResponse, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "get_trust")
	defer span.End()

	trust, err := s.trust.Get(ctx, req.GetTenantId())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get an oidc provider")
		return nil, fmt.Errorf("getting odic provider: %w", err)
	}

	span.SetStatus(codes.Ok, "")
	return &sessionv1.GetTrustResponse{Trust: trust}, nil
}

func (s *Server) getClientID(oidcTrust *oidcv1.OIDC) string {
	if clientID := oidcTrust.GetClientId(); clientID != "" {
		return clientID
	}

	return s.clientID
}

func (s *Server) httpClient(oidcTrust *oidcv1.OIDC) *http.Client {
	creds := s.newCreds(s.getClientID(oidcTrust))
	transport := creds.Transport()
	if debugSettingSMDumpTransport.Value() == "1" {
		transport = debugtools.NewTransport(transport)
	}

	return &http.Client{
		Transport: transport,
	}
}

func (s *Server) introspectToken(ctx context.Context, token string, oidcTrust *oidcv1.OIDC) (oidc.Introspection, error) {
	// first check the cache for a recent introspection result for this token
	hashedSuffix := sha256.Sum256([]byte(token))
	cacheKey := base64.RawURLEncoding.EncodeToString(hashedSuffix[:])
	if item := s.introspectionCache.Get(cacheKey); item != nil {
		return item.Value(), nil
	}

	httpClient := s.httpClient(oidcTrust)

	// create the provider for the given issuer
	provider, err := oidc.NewProvider(oidcTrust.GetIssuer(), oidcTrust.GetAudiences(),
		oidc.WithAllowHttpScheme(s.allowHttpScheme),
		oidc.WithSecureHTTPClient(httpClient),
	)
	if err != nil {
		slogctx.Error(ctx, "Could not create OpenID provider", "issuer", oidcTrust.GetIssuer(), "error", err)
		return oidc.Introspection{Active: false}, err
	}

	// introspect the token
	intr, err := provider.IntrospectToken(ctx, token)
	if err != nil {
		if errors.Is(err, oidc.ErrNoIntrospectionEndpoint) {
			slogctx.Debug(ctx, "No introspection endpoint configured", "issuer", provider.Issuer)
			return oidc.Introspection{Active: true}, nil
		}
		slogctx.Error(ctx, "Could not introspect token", "error", err)
		return oidc.Introspection{Active: false}, err
	}

	// Cache the result with TTL
	s.introspectionCache.Set(cacheKey, intr, ttlcache.DefaultTTL)

	return intr, nil
}
