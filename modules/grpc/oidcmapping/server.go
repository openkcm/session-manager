//nolint:staticcheck // Ignore deprecation checks as we implement a compatibility layer with deprecated API
package oidcmapping

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"
	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/pkg/serviceerr"
)

type Server struct {
	oidcmappingv1.UnimplementedServiceServer

	trust sessionmanager.Trust
}

func NewServer(trust sessionmanager.Trust) *Server {
	return &Server{trust: trust}
}

func (srv *Server) ApplyOIDCMapping(ctx context.Context, req *oidcmappingv1.ApplyOIDCMappingRequest) (*oidcmappingv1.ApplyOIDCMappingResponse, error) {
	oidcBuilder := oidcv1.OIDC_builder{
		Issuer:    new(req.GetIssuer()),
		Audiences: req.GetAudiences(),
	}
	if req.JwksUri != nil {
		oidcBuilder.JwksUri = new(req.GetJwksUri())
	}
	if req.ClientId != nil {
		oidcBuilder.ClientId = new(req.GetClientId())
	}
	oidc := oidcBuilder.Build()

	trust := trustv1.Trust_builder{
		TenantId: new(req.GetTenantId()),
		Oidc:     oidc,
	}.Build()

	ctx = slogctx.With(ctx,
		"tenantId", trust.GetTenantId(),
		"issuer", oidc.GetIssuer(),
		"jwksUri", oidc.GetJwksUri(),
		"audiences", oidc.GetAudiences(),
		"client_id", oidc.GetClientId(),
	)

	slogctx.Debug(ctx, "ApplyOIDCMapping called")

	response := &oidcmappingv1.ApplyOIDCMappingResponse{}

	if err := srv.trust.Apply(ctx, trust); err != nil {
		slogctx.Error(ctx, "Could not apply trust", "error", err)
		if errors.Is(err, serviceerr.ErrNotFound) {
			msg := serviceerr.ErrNotFound.Error()
			response.Message = &msg
			return response, nil
		}

		return nil, status.Errorf(codes.Internal, "failed to apply trust: %v", err)
	}

	response.Success = true
	return response, nil
}

func (srv *Server) RemoveOIDCMapping(ctx context.Context, req *oidcmappingv1.RemoveOIDCMappingRequest) (*oidcmappingv1.RemoveOIDCMappingResponse, error) {
	ctx = slogctx.With(ctx, "tenantId", req.GetTenantId())
	slogctx.Debug(ctx, "RemoveOIDCMapping called")

	resp := &oidcmappingv1.RemoveOIDCMappingResponse{}
	if err := srv.trust.Remove(ctx, req.GetTenantId()); err != nil {
		if !errors.Is(err, serviceerr.ErrNotFound) {
			slogctx.Error(ctx, "Could not remove trust", "error", err)
			msg := err.Error()
			resp.Message = &msg
			return resp, status.Error(codes.Internal, "failed to remove trust: "+msg)
		}
		slogctx.Warn(ctx, "RemoveOIDCMapping is called but the tenant does not exist", "error", err)
	}

	resp.Success = true
	return resp, nil
}

func (srv *Server) BlockOIDCMapping(ctx context.Context, req *oidcmappingv1.BlockOIDCMappingRequest) (*oidcmappingv1.BlockOIDCMappingResponse, error) {
	ctx = slogctx.With(ctx, "tenantId", req.GetTenantId())
	slogctx.Debug(ctx, "BlockOIDCMapping called")

	resp := &oidcmappingv1.BlockOIDCMappingResponse{}
	if err := srv.trust.Block(ctx, req.GetTenantId()); err != nil {
		slogctx.Error(ctx, "Could not block trust", "error", err)
		msg := err.Error()
		resp.Message = &msg
		return resp, status.Error(codes.Internal, "failed to block trust: "+msg)
	}

	resp.Success = true
	return resp, nil
}

func (srv *Server) UnblockOIDCMapping(ctx context.Context, req *oidcmappingv1.UnblockOIDCMappingRequest) (*oidcmappingv1.UnblockOIDCMappingResponse, error) {
	ctx = slogctx.With(ctx, "tenantId", req.GetTenantId())
	slogctx.Debug(ctx, "UnblockOIDCMapping called")

	resp := &oidcmappingv1.UnblockOIDCMappingResponse{}
	if err := srv.trust.Unblock(ctx, req.GetTenantId()); err != nil {
		slogctx.Error(ctx, "Could not unblock trust", "error", err)
		msg := err.Error()
		resp.Message = &msg
		return resp, status.Error(codes.Internal, "failed to unblock trust: "+msg)
	}

	resp.Success = true
	return resp, nil
}
