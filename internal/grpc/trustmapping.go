package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"
	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"
	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/pkg/serviceerr"
)

type TrustMappingServer struct {
	trustmappingv1.UnimplementedServiceServer

	trust sessionmanager.Trust
}

func NewTrustMappingServer(trust sessionmanager.Trust) *TrustMappingServer {
	srv := &TrustMappingServer{
		trust: trust,
	}

	return srv
}

func (srv *TrustMappingServer) ApplyTrustMapping(ctx context.Context, in *trustmappingv1.ApplyTrustMappingRequest) (*trustmappingv1.ApplyTrustMappingResponse, error) {
	oidcIn := in.GetOidc()
	oidc := oidcv1.OIDC_builder{
		Issuer:    new(oidcIn.GetIssuer()),
		JwksUri:   new(oidcIn.GetJwksUri()),
		Audiences: oidcIn.GetAudiences(),
		ClientId:  new(oidcIn.GetClientId()),
	}.Build()

	trust := trustv1.Trust_builder{
		TenantId: new(in.GetTenantId()),
		Oidc:     oidc,
	}.Build()

	ctx = slogctx.With(ctx,
		"tenantId", trust.GetTenantId(),
		"issuer", oidc.GetIssuer(),
		"jwksUri", oidc.GetJwksUri(),
		"audiences", oidc.GetAudiences(),
		"client_id", oidc.GetClientId(),
	)

	slogctx.Debug(ctx, "ApplyTrustMapping called")

	response := trustmappingv1.ApplyTrustMappingResponse_builder{}.Build()

	if err := srv.trust.ApplyMapping(ctx, trust); err != nil {
		slogctx.Error(ctx, "Could not apply Trust mapping", "error", err)
		if errors.Is(err, serviceerr.ErrNotFound) {
			msg := serviceerr.ErrNotFound.Error()
			response.SetMessage(msg)
			return response, nil
		}

		return nil, status.Errorf(codes.Internal, "failed to apply Trust mapping: %v", err)
	}

	response.SetSuccess(true)

	return response, nil
}

// BlockTrustMapping blocks the Trust mapping for the specified tenant.
// It calls the underlying service to set the mapping as blocked.
// Returns a response containing an optional error message if blocking fails.
func (srv *TrustMappingServer) BlockTrustMapping(ctx context.Context, req *trustmappingv1.BlockTrustMappingRequest) (*trustmappingv1.BlockTrustMappingResponse, error) {
	ctx = slogctx.With(ctx, "tenantId", req.GetTenantId())
	slogctx.Debug(ctx, "BlockTrustMapping called")

	resp := trustmappingv1.BlockTrustMappingResponse_builder{}.Build()
	err := srv.trust.BlockMapping(ctx, req.GetTenantId())
	if err != nil {
		slogctx.Error(ctx, "Could not block Trust mapping", "error", err)
		msg := err.Error()

		resp.SetMessage(msg)
		return resp, status.Error(codes.Internal, "failed to block Trust mapping: "+msg)
	}

	resp.SetSuccess(true)
	return resp, nil
}

// RemoveTrustMapping removes the Trust configuration for the tenant.
// It calls the underlying service to remove the mapping.
// Returns a respose containing an optional error message if removing fails.
func (srv *TrustMappingServer) RemoveTrustMapping(ctx context.Context, req *trustmappingv1.RemoveTrustMappingRequest) (*trustmappingv1.RemoveTrustMappingResponse, error) {
	ctx = slogctx.With(ctx, "tenantId", req.GetTenantId())
	slogctx.Debug(ctx, "RemoveTrustMapping called")

	resp := &trustmappingv1.RemoveTrustMappingResponse{}
	err := srv.trust.RemoveMapping(ctx, req.GetTenantId())
	if err != nil {
		if !errors.Is(err, serviceerr.ErrNotFound) {
			slogctx.Error(ctx, "Could not remove Trust mapping", "error", err)
			msg := err.Error()
			resp.SetMessage(msg)
			return resp, status.Error(codes.Internal, "failed to remove Trust mapping: "+msg)
		} else {
			slogctx.Warn(ctx, "RemoveTrustMapping is called but the tenant does not exist", "error", err)
		}
	}

	resp.SetSuccess(true)
	return resp, nil
}

// UnblockTrustMapping unblocks the Trust mapping for the specified tenant.
// It calls the underlying service to set the mapping as unblocked.
// Returns a response containing an optional error message if unblocking fails.
func (srv *TrustMappingServer) UnblockTrustMapping(ctx context.Context, req *trustmappingv1.UnblockTrustMappingRequest) (*trustmappingv1.UnblockTrustMappingResponse, error) {
	ctx = slogctx.With(ctx, "tenantId", req.GetTenantId())
	slogctx.Debug(ctx, "UnblockTrustMapping called")

	resp := &trustmappingv1.UnblockTrustMappingResponse{}
	err := srv.trust.UnblockMapping(ctx, req.GetTenantId())
	if err != nil {
		slogctx.Error(ctx, "Could not unblock Trust mapping", "error", err)
		msg := err.Error()
		resp.SetMessage(msg)
		return resp, status.Error(codes.Internal, "failed to unblock Trust mapping: "+msg)
	}

	resp.SetSuccess(true)
	return resp, nil
}
