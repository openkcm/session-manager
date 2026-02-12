package grpc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/openkcm/common-sdk/pkg/openid"

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

type SessionServer struct {
	sessionv1.UnimplementedServiceServer

	sessionRepo session.Repository
	trustRepo   trust.OIDCMappingRepository
	httpClient  *http.Client

	queryParametersIntrospect []string
	idleSessionTimeout        time.Duration
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

	// Get trust mapping for the given tenant ID
	mapping, err := s.trustRepo.Get(ctx, req.GetTenantId())
	if err != nil {
		slogctx.Warn(ctx, "Is this an attack? Could not get trust mapping", "issuer", sess.Issuer, "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}
	if mapping.Blocked {
		slogctx.Warn(ctx, "Tenant is blocked", "issuer", sess.Issuer)
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

	// Introspect access token
	cfg, err := openid.GetConfig(ctx, mapping.IssuerURL)
	if err != nil {
		slogctx.Error(ctx, "Could not get OpenID configuration", "issuer", sess.Issuer, "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, err
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

	if cfg.IntrospectionEndpoint != "" {
		result, err := cfg.IntrospectToken(ctx, sess.AccessToken, mapping.GetIntrospectParameters(s.queryParametersIntrospect))
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
	}

	if err := s.sessionRepo.BumpActive(ctx, req.GetSessionId(), s.idleSessionTimeout); err != nil {
		slogctx.Error(ctx, "failed to bump the session status", "error", err)
		return &sessionv1.GetSessionResponse{Valid: false}, nil
	}

	// Return info of the valid session
	return response, nil
}

func (s *SessionServer) GetOIDCProvider(ctx context.Context, req *sessionv1.GetOIDCProviderRequest) (*sessionv1.GetOIDCProviderResponse, error) {
	provider, err := s.trustRepo.Get(ctx, req.GetTenantId())
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
