package grpc_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"
	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	"github.com/openkcm/session-manager/internal/grpc"
	mocktrust "github.com/openkcm/session-manager/modules/oidctrust/mocks"
	"github.com/openkcm/session-manager/pkg/serviceerr"
)

func TestNewTrustMappingServer(t *testing.T) {
	t.Run("creates server successfully", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository()
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		assert.NotNil(t, server)
	})
}

func TestApplyTrustMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success - creates new trust", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository()
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		jwksUri := "https://issuer.example.com/.well-known/jwks.json"
		req := trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    new("https://issuer.example.com"),
				JwksUri:   &jwksUri,
				Audiences: []string{"audience1", "audience2"},
			}.Build(),
		}.Build()

		resp, err := server.ApplyTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("success - updates existing trust", func(t *testing.T) {
		existingTrust := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Oidc: oidcv1.OIDC_builder{
				Issuer:    new("https://old-issuer.example.com"),
				JwksUri:   new("https://old-issuer.example.com/jwks.json"),
				Audiences: []string{"old-audience"},
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithTrust(existingTrust),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		jwksUri := "https://new-issuer.example.com/jwks.json"
		req := trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    new("https://new-issuer.example.com"),
				JwksUri:   new(jwksUri),
				Audiences: []string{"new-audience"},
			}.Build(),
		}.Build()

		resp, err := server.ApplyTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("not found error - returns response with message", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithCreateError(serviceerr.ErrNotFound),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		jwksUri := "https://issuer.example.com/jwks.json"
		req := trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:  new("https://issuer.example.com"),
				JwksUri: new(jwksUri),
			}.Build(),
		}.Build()

		resp, err := server.ApplyTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetSuccess())
		require.NotNil(t, resp.GetMessage())
		assert.Equal(t, serviceerr.ErrNotFound.Error(), resp.GetMessage())
	})

	t.Run("internal error - returns grpc error", func(t *testing.T) {
		internalErr := errors.New("database connection failed")
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithCreateError(internalErr),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		jwksUri := "https://issuer.example.com/jwks.json"
		req := trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:  new("https://issuer.example.com"),
				JwksUri: new(jwksUri),
			}.Build(),
		}.Build()

		resp, err := server.ApplyTrustMapping(ctx, req)

		assert.Nil(t, resp)
		require.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to apply trust")
	})

	t.Run("update error - returns grpc error", func(t *testing.T) {
		existingTrust := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		updateErr := errors.New("update failed")
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithTrust(existingTrust),
			mocktrust.WithUpdateError(updateErr),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		jwksUri := "https://new-issuer.example.com/jwks.json"
		req := trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:  new("https://new-issuer.example.com"),
				JwksUri: new(jwksUri),
			}.Build(),
		}.Build()

		resp, err := server.ApplyTrustMapping(ctx, req)

		assert.Nil(t, resp)
		require.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestBlockTrustMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success - blocks existing trust", func(t *testing.T) {
		existingTrust := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Blocked:  new(false),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithTrust(existingTrust),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.BlockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("success - already blocked", func(t *testing.T) {
		existingTrust := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Blocked:  new(true),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithTrust(existingTrust),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.BlockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("not found - returns success", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithGetError(serviceerr.ErrNotFound),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.BlockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("error - returns grpc error with message", func(t *testing.T) {
		internalErr := errors.New("database error")
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithGetError(internalErr),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.BlockTrustMapping(ctx, req)

		require.Error(t, err)
		assert.NotNil(t, resp)
		require.NotNil(t, resp.GetMessage())
		assert.Contains(t, resp.GetMessage(), "database error")

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to block trust")
	})
}

func TestRemoveTrustMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success - removes existing trust", func(t *testing.T) {
		existingTrust := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithTrust(existingTrust),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.RemoveTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.RemoveTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("error - returns grpc error with message", func(t *testing.T) {
		deleteErr := errors.New("delete failed")
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithDeleteError(deleteErr),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.RemoveTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.RemoveTrustMapping(ctx, req)

		require.Error(t, err)
		assert.NotNil(t, resp)
		require.NotNil(t, resp.GetMessage())
		assert.Contains(t, resp.GetMessage(), "delete failed")

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to remove trust")
	})

	t.Run("error - delete is indempotent", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithDeleteError(serviceerr.ErrNotFound),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.RemoveTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.RemoveTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})
}

func TestUnblockTrustMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success - unblocks blocked trust", func(t *testing.T) {
		existingTrust := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Blocked:  new(true),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithTrust(existingTrust),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.UnblockTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.UnblockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("success - already unblocked", func(t *testing.T) {
		existingTrust := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Blocked:  new(false),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithTrust(existingTrust),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.UnblockTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.UnblockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("not found - returns success", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithGetError(serviceerr.ErrNotFound),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.UnblockTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.UnblockTrustMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("error - returns grpc error with message", func(t *testing.T) {
		internalErr := errors.New("update failed")
		existingTrust := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Blocked:  new(true),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithTrust(existingTrust),
			mocktrust.WithUpdateError(internalErr),
		)
		svc := newTrust(repo)
		server := grpc.NewTrustMappingServer(svc)

		req := trustmappingv1.UnblockTrustMappingRequest_builder{
			TenantId: new("tenant-123"),
		}.Build()

		resp, err := server.UnblockTrustMapping(ctx, req)

		require.Error(t, err)
		assert.NotNil(t, resp)
		require.NotNil(t, resp.GetMessage())
		assert.Contains(t, resp.GetMessage(), "update failed")

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to unblock trust")
	})
}
