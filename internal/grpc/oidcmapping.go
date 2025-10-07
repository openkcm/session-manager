package grpc

import (
	"context"
	"errors"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	"github.com/openkcm/session-manager/internal/oidc"
)

type OIDCMappingServer struct {
	oidcmappingv1.UnimplementedOIDCMappingServer

	repo oidc.ProviderRepository
}

func NewOIDCMappingServer(repo oidc.ProviderRepository) *OIDCMappingServer {
	srv := &OIDCMappingServer{
		repo: repo,
	}
	return srv
}

func (srv *OIDCMappingServer) ApplyOIDCMapping(context.Context, *oidcmappingv1.ApplyOIDCMappingRequest) (*oidcmappingv1.ApplyOIDCMappingResponse, error) {
	// TODO: Implement the logic to create or update OIDC mappings in the repository.
	return nil, errors.New("not implemented")
}
func (srv *OIDCMappingServer) RemoveOIDCMapping(context.Context, *oidcmappingv1.RemoveOIDCMappingRequest) (*oidcmappingv1.RemoveOIDCMappingResponse, error) {
	// TODO: Implement the logic to remove OIDC mappings from the repository.
	return nil, errors.New("not implemented")
}
