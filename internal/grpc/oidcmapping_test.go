package grpc_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	"github.com/openkcm/session-manager/internal/grpc"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/trust"
	"github.com/openkcm/session-manager/internal/trust/trustmock"
)

func TestNewOIDCMappingServer(t *testing.T) {
	t.Run("creates server successfully", func(t *testing.T) {
		repo := trustmock.NewInMemRepository()
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		assert.NotNil(t, server)
	})
}

func TestApplyOIDCMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success - creates new mapping", func(t *testing.T) {
		repo := trustmock.NewInMemRepository()
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		jwksUri := "https://issuer.example.com/.well-known/jwks.json"
		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  "tenant-123",
			Issuer:    "https://issuer.example.com",
			JwksUri:   &jwksUri,
			Audiences: []string{"audience1", "audience2"},
			Properties: map[string]string{
				"prop1": "value1",
				"prop2": "value2",
			},
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("success - updates existing mapping", func(t *testing.T) {
		existingProvider := trust.Provider{
			IssuerURL: "https://old-issuer.example.com",
			JWKSURI:   "https://old-issuer.example.com/jwks.json",
			Audiences: []string{"old-audience"},
		}
		repo := trustmock.NewInMemRepository(
			trustmock.WithTrust("tenant-123", existingProvider),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		jwksUri := "https://new-issuer.example.com/jwks.json"
		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  "tenant-123",
			Issuer:    "https://new-issuer.example.com",
			JwksUri:   &jwksUri,
			Audiences: []string{"new-audience"},
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("not found error - returns response with message", func(t *testing.T) {
		repo := trustmock.NewInMemRepository(
			trustmock.WithCreateError(serviceerr.ErrNotFound),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		jwksUri := "https://issuer.example.com/jwks.json"
		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId: "tenant-123",
			Issuer:   "https://issuer.example.com",
			JwksUri:  &jwksUri,
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetSuccess())
		require.NotNil(t, resp.GetMessage())
		assert.Equal(t, serviceerr.ErrNotFound.Error(), resp.GetMessage())
	})

	t.Run("internal error - returns grpc error", func(t *testing.T) {
		internalErr := errors.New("database connection failed")
		repo := trustmock.NewInMemRepository(
			trustmock.WithCreateError(internalErr),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		jwksUri := "https://issuer.example.com/jwks.json"
		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId: "tenant-123",
			Issuer:   "https://issuer.example.com",
			JwksUri:  &jwksUri,
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)

		assert.Nil(t, resp)
		require.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to apply OIDC mapping")
	})

	t.Run("update error - returns grpc error", func(t *testing.T) {
		existingProvider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
		}
		updateErr := errors.New("update failed")
		repo := trustmock.NewInMemRepository(
			trustmock.WithTrust("tenant-123", existingProvider),
			trustmock.WithUpdateError(updateErr),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		jwksUri := "https://new-issuer.example.com/jwks.json"
		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId: "tenant-123",
			Issuer:   "https://new-issuer.example.com",
			JwksUri:  &jwksUri,
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)

		assert.Nil(t, resp)
		require.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestBlockOIDCMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success - blocks existing mapping", func(t *testing.T) {
		existingProvider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
			Blocked:   false,
		}
		repo := trustmock.NewInMemRepository(
			trustmock.WithTrust("tenant-123", existingProvider),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.BlockOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.BlockOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("success - already blocked", func(t *testing.T) {
		existingProvider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
			Blocked:   true,
		}
		repo := trustmock.NewInMemRepository(
			trustmock.WithTrust("tenant-123", existingProvider),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.BlockOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.BlockOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("not found - returns success", func(t *testing.T) {
		repo := trustmock.NewInMemRepository(
			trustmock.WithGetError(serviceerr.ErrNotFound),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.BlockOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.BlockOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("error - returns grpc error with message", func(t *testing.T) {
		internalErr := errors.New("database error")
		repo := trustmock.NewInMemRepository(
			trustmock.WithGetError(internalErr),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.BlockOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.BlockOIDCMapping(ctx, req)

		require.Error(t, err)
		assert.NotNil(t, resp)
		require.NotNil(t, resp.GetMessage())
		assert.Contains(t, resp.GetMessage(), "database error")

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to block OIDC mapping")
	})
}

func TestRemoveOIDCMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success - removes existing mapping", func(t *testing.T) {
		existingProvider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
		}
		repo := trustmock.NewInMemRepository(
			trustmock.WithTrust("tenant-123", existingProvider),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.RemoveOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.RemoveOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("error - returns grpc error with message", func(t *testing.T) {
		deleteErr := errors.New("delete failed")
		repo := trustmock.NewInMemRepository(
			trustmock.WithDeleteError(deleteErr),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.RemoveOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.RemoveOIDCMapping(ctx, req)

		require.Error(t, err)
		assert.NotNil(t, resp)
		require.NotNil(t, resp.GetMessage())
		assert.Contains(t, resp.GetMessage(), "delete failed")

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to remove OIDC mapping")
	})

	t.Run("error - delete is indempotent", func(t *testing.T) {
		repo := trustmock.NewInMemRepository(
			trustmock.WithDeleteError(serviceerr.ErrNotFound),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.RemoveOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.RemoveOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})
}

func TestUnblockOIDCMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success - unblocks blocked mapping", func(t *testing.T) {
		existingProvider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
			Blocked:   true,
		}
		repo := trustmock.NewInMemRepository(
			trustmock.WithTrust("tenant-123", existingProvider),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.UnblockOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.UnblockOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("success - already unblocked", func(t *testing.T) {
		existingProvider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
			Blocked:   false,
		}
		repo := trustmock.NewInMemRepository(
			trustmock.WithTrust("tenant-123", existingProvider),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.UnblockOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.UnblockOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("not found - returns success", func(t *testing.T) {
		repo := trustmock.NewInMemRepository(
			trustmock.WithGetError(serviceerr.ErrNotFound),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.UnblockOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.UnblockOIDCMapping(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("error - returns grpc error with message", func(t *testing.T) {
		internalErr := errors.New("update failed")
		existingProvider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
			Blocked:   true,
		}
		repo := trustmock.NewInMemRepository(
			trustmock.WithTrust("tenant-123", existingProvider),
			trustmock.WithUpdateError(internalErr),
		)
		svc := trust.NewService(repo)
		server := grpc.NewOIDCMappingServer(svc)

		req := &oidcmappingv1.UnblockOIDCMappingRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.UnblockOIDCMapping(ctx, req)

		require.Error(t, err)
		assert.NotNil(t, resp)
		require.NotNil(t, resp.GetMessage())
		assert.Contains(t, resp.GetMessage(), "update failed")

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to unblock OIDC mapping")
	})
}
