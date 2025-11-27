package grpc

import (
	oidcproviderv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcprovider/v1"

	"github.com/openkcm/session-manager/internal/oidc"
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
