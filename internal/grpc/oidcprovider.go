package grpc

import (
	"context"
	"errors"

	oidcproviderv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcprovider/v1"

	"github.com/openkcm/session-manager/internal/oidc"
)

type OIDCProviderServer struct {
	oidcproviderv1.UnimplementedOIDCProviderServer

	repo oidc.ProviderRepository
}

func NewOIDCProviderServer(repo oidc.ProviderRepository) *OIDCProviderServer {
	srv := &OIDCProviderServer{
		repo: repo,
	}
	return srv
}

func (srv *OIDCProviderServer) GetOIDCProvider(context.Context, *oidcproviderv1.GetOIDCProviderRequest) (*oidcproviderv1.GetOIDCProviderResponse, error) {
	// TODO: Implement the logic to get OIDC provider details from the repository.
	return nil, errors.New("not implemented")
}
