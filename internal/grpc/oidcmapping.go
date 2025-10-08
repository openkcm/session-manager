package grpc

import (
	"context"
	"fmt"
	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openkcm/session-manager/internal/oidc"
)

type OIDCMappingServer struct {
	oidcmappingv1.UnimplementedOIDCMappingServer

	oidc *oidc.Service
}

func NewOIDCMappingServer(oidc *oidc.Service) *OIDCMappingServer {
	srv := &OIDCMappingServer{
		oidc: oidc,
	}

	return srv
}

func (srv *OIDCMappingServer) ApplyOIDCMapping(ctx context.Context, req *oidcmappingv1.ApplyOIDCMappingRequest) (*oidcmappingv1.ApplyOIDCMappingResponse, error) {
	response := &oidcmappingv1.ApplyOIDCMappingResponse{
		Success: false,
	}
	provider, err := srv.oidc.GetForTenant(ctx, req.TenantId)
	if err != nil {
		provider = oidc.Provider{
			IssuerURL: req.Issuer,
			Blocked:   req.Blocked,
			JWKSURIs:  req.JwksUris,
			Audiences: req.Audiences,
		}
		err = srv.oidc.Create(ctx, req.TenantId, provider)
		if err != nil {
			msg := err.Error()
			response.Message = &msg

			return response, status.Error(codes.Internal, fmt.Sprintf("failed to apply OIDC mapping: %s", msg))
		}
	} else {
		provider = oidc.Provider{
			IssuerURL: req.Issuer,
			Blocked:   req.Blocked,
			JWKSURIs:  req.JwksUris,
			Audiences: req.Audiences,
		}
		err = srv.oidc.Update(ctx, req.TenantId, provider)
		if err != nil {
			msg := err.Error()
			response.Message = &msg

			return response, status.Error(codes.Internal, fmt.Sprintf("failed to apply OIDC mapping: %s", msg))
		}
	}
	response.Success = true

	return response, nil
}

func (srv *OIDCMappingServer) RemoveOIDCMapping(ctx context.Context, req *oidcmappingv1.RemoveOIDCMappingRequest) (*oidcmappingv1.RemoveOIDCMappingResponse, error) {
	resp := &oidcmappingv1.RemoveOIDCMappingResponse{
		Success: false,
	}
	provider, err := srv.oidc.GetForTenant(ctx, req.TenantId)
	if err != nil {
		msg := err.Error()
		resp.Message = &msg
		return resp, status.Error(codes.NotFound, fmt.Sprintf("provider for tenant not found: %s", msg))
	}
	err = srv.oidc.Delete(ctx, req.TenantId, provider)
	if err != nil {
		msg := err.Error()
		resp.Message = &msg
		return resp, status.Error(codes.Internal, fmt.Sprintf("delete failed: %s", msg))
	}
	resp.Success = true

	return resp, nil
}
