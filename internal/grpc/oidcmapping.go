package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

type OIDCMappingServer struct {
	oidcmappingv1.UnimplementedServiceServer

	oidc *oidc.Service
}

func NewOIDCMappingServer(oidc *oidc.Service) *OIDCMappingServer {
	srv := &OIDCMappingServer{
		oidc: oidc,
	}

	return srv
}

func (srv *OIDCMappingServer) ApplyOIDCMapping(ctx context.Context, req *oidcmappingv1.ApplyOIDCMappingRequest) (*oidcmappingv1.ApplyOIDCMappingResponse, error) {
	slogctx.Debug(ctx, "ApplyOIDCMapping called",
		"tenant_id", req.GetTenantId(),
		"issuer", req.GetIssuer(),
		"jwks_uris", req.GetJwksUris(),
		"audiences", req.GetAudiences(),
		"properties", req.GetProperties(),
	)

	response := &oidcmappingv1.ApplyOIDCMappingResponse{}
	provider := oidc.Provider{
		IssuerURL:  req.GetIssuer(),
		Blocked:    false,
		JWKSURIs:   req.GetJwksUris(),
		Audiences:  req.GetAudiences(),
		Properties: req.GetProperties(),
	}
	err := srv.oidc.ApplyMapping(ctx, req.GetTenantId(), provider)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			msg := serviceerr.ErrNotFound.Error()
			response.Message = &msg
			return response, nil
		}

		return nil, status.Errorf(codes.Internal, "failed to apply OIDC mapping: %v", err)
	}

	response.Success = true

	return response, nil
}

func (srv *OIDCMappingServer) RemoveOIDCMapping(ctx context.Context, req *oidcmappingv1.RemoveOIDCMappingRequest) (*oidcmappingv1.RemoveOIDCMappingResponse, error) {
	slogctx.Debug(ctx, "RemoveOIDCMapping called",
		"tenant_id", req.GetTenantId(),
	)

	resp := &oidcmappingv1.RemoveOIDCMappingResponse{}
	err := srv.oidc.RemoveMapping(ctx, req.GetTenantId())
	if err != nil {
		msg := err.Error()
		resp.Message = &msg
		return resp, status.Error(codes.Internal, "failed to remove OIDC mapping: "+msg)
	}

	resp.Success = true

	return resp, nil
}

// BlockOIDCMapping blocks the OIDC mapping for the specified tenant.
// It calls the underlying service to set the mapping as blocked.
// Returns a response containing an optional error message if blocking fails.
func (srv *OIDCMappingServer) BlockOIDCMapping(ctx context.Context, req *oidcmappingv1.BlockOIDCMappingRequest) (*oidcmappingv1.BlockOIDCMappingResponse, error) {
	slogctx.Debug(ctx, "BlockOIDCMapping called",
		"tenant_id", req.GetTenantId(),
	)

	resp := &oidcmappingv1.BlockOIDCMappingResponse{}
	err := srv.oidc.BlockMapping(ctx, req.GetTenantId())
	if err != nil {
		msg := err.Error()
		resp.Message = &msg
		return resp, status.Error(codes.Internal, "failed to block OIDC mapping: "+msg)
	}
	resp.Success = true
	return resp, nil
}

// UnblockOIDCMapping unblocks the OIDC mapping for the specified tenant.
// It calls the underlying service to set the mapping as unblocked.
// Returns a response containing an optional error message if unblocking fails.
func (srv *OIDCMappingServer) UnblockOIDCMapping(ctx context.Context, req *oidcmappingv1.UnblockOIDCMappingRequest) (*oidcmappingv1.UnblockOIDCMappingResponse, error) {
	slogctx.Debug(ctx, "UnblockOIDCMapping called",
		"tenant_id", req.GetTenantId(),
	)

	resp := &oidcmappingv1.UnblockOIDCMappingResponse{}
	err := srv.oidc.UnBlockMapping(ctx, req.GetTenantId())
	if err != nil {
		msg := err.Error()
		resp.Message = &msg
		return resp, status.Error(codes.Internal, "failed to unblock OIDC mapping: "+msg)
	}
	resp.Success = true
	return resp, nil
}
