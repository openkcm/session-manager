package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	oidcproviderv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcprovider/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

type OIDCProviderServer struct {
	oidcproviderv1.UnimplementedServiceServer

	oidc *oidc.Service
}

func NewOIDCProviderServer(oidcService *oidc.Service) *OIDCProviderServer {
	return &OIDCProviderServer{
		oidc: oidcService,
	}
}

func (s *OIDCProviderServer) GetOIDCProvider(ctx context.Context, req *oidcproviderv1.GetOIDCProviderRequest) (*oidcproviderv1.GetOIDCProviderResponse, error) {
	slogctx.Debug(ctx, "GetOIDCProvider called",
		"issuer", req.GetIssuer(),
	)

	provider, err := s.oidc.GetProvider(ctx, req.GetIssuer())
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "oidc provider not found")
		}

		slogctx.Error(ctx, "failed to get provider", "error", err)
		return nil, status.Error(codes.Internal, "failed to get provider")
	}

	return &oidcproviderv1.GetOIDCProviderResponse{
		Issuer:    provider.IssuerURL,
		JwksUris:  provider.JWKSURIs,
		Audiences: provider.Audiences,
	}, nil
}
