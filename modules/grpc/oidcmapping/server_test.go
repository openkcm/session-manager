//nolint:staticcheck // Ignore deprecation checks as we implement a compatibility layer with deprecated API
package oidcmapping_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	"github.com/openkcm/session-manager/modules/grpc/oidcmapping"
	mocktrust "github.com/openkcm/session-manager/modules/oidctrust/mocks"
	"github.com/openkcm/session-manager/pkg/serviceerr"
)

func TestNewOIDCMappingServer(t *testing.T) {
	repo := mocktrust.NewInMemRepository()
	svc := newTrust(repo)
	server := oidcmapping.NewServer(svc)
	assert.NotNil(t, server)
}

func TestApplyOIDCMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("forwards issuer, jwks_uri, audiences, client_id when set", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository()
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		jwksURI := "https://issuer.example.com/.well-known/jwks.json"
		clientID := "client-abc"
		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  "tenant-123",
			Issuer:    "https://issuer.example.com",
			JwksUri:   &jwksURI,
			Audiences: []string{"audience1", "audience2"},
			ClientId:  &clientID,
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())

		stored := repo.TGet("tenant-123")
		require.NotNil(t, stored)
		assert.Equal(t, "tenant-123", stored.GetTenantId())
		require.NotNil(t, stored.GetOidc())
		assert.Equal(t, "https://issuer.example.com", stored.GetOidc().GetIssuer())
		assert.Equal(t, jwksURI, stored.GetOidc().GetJwksUri())
		assert.Equal(t, []string{"audience1", "audience2"}, stored.GetOidc().GetAudiences())
		assert.Equal(t, clientID, stored.GetOidc().GetClientId())
		assert.True(t, stored.GetOidc().HasClientId())
	})

	t.Run("client_id omitted leaves new oidc.client_id unset", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository()
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId: "tenant-no-client",
			Issuer:   "https://issuer.example.com",
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())

		stored := repo.TGet("tenant-no-client")
		require.NotNil(t, stored)
		require.NotNil(t, stored.GetOidc())
		assert.False(t, stored.GetOidc().HasClientId(), "client_id should remain unset when request omits it")
	})

	t.Run("non-empty properties map is dropped", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository()
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		clientID := "client-xyz"
		reqWithProps := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:   "tenant-with-props",
			Issuer:     "https://issuer.example.com",
			ClientId:   &clientID,
			Properties: map[string]string{"foo": "bar", "baz": "qux"},
		}

		resp, err := server.ApplyOIDCMapping(ctx, reqWithProps)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())

		stored := repo.TGet("tenant-with-props")
		require.NotNil(t, stored)
		// The new oidc.OIDC has no properties field; verify the stored Trust matches what
		// we'd get by building it from the same request without properties.
		expected := trustv1.Trust_builder{
			TenantId: new("tenant-with-props"),
			Oidc: oidcv1.OIDC_builder{
				Issuer:   new("https://issuer.example.com"),
				ClientId: new(clientID),
			}.Build(),
		}.Build()
		assert.Equal(t, expected.GetTenantId(), stored.GetTenantId())
		assert.Equal(t, expected.GetOidc().GetIssuer(), stored.GetOidc().GetIssuer())
		assert.Equal(t, expected.GetOidc().GetClientId(), stored.GetOidc().GetClientId())
	})

	t.Run("ErrNotFound from Apply yields non-success response with message and no gRPC error", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithCreateError(serviceerr.ErrNotFound),
		)
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId: "tenant-missing",
			Issuer:   "https://issuer.example.com",
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, resp.GetSuccess())
		assert.Equal(t, serviceerr.ErrNotFound.Error(), resp.GetMessage())
	})

	t.Run("other errors map to codes.Internal", func(t *testing.T) {
		internalErr := errors.New("database connection failed")
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithCreateError(internalErr),
		)
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId: "tenant-boom",
			Issuer:   "https://issuer.example.com",
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)
		assert.Nil(t, resp)
		require.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to apply trust")
	})
}

func TestRemoveOIDCMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success removes existing trust", func(t *testing.T) {
		existing := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(mocktrust.WithTrust(existing))
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.RemoveOIDCMappingRequest{TenantId: "tenant-123"}
		resp, err := server.RemoveOIDCMapping(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("ErrNotFound is idempotent and returns success", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithDeleteError(serviceerr.ErrNotFound),
		)
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.RemoveOIDCMappingRequest{TenantId: "tenant-gone"}
		resp, err := server.RemoveOIDCMapping(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("other errors map to codes.Internal", func(t *testing.T) {
		deleteErr := errors.New("delete failed")
		repo := mocktrust.NewInMemRepository(mocktrust.WithDeleteError(deleteErr))
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.RemoveOIDCMappingRequest{TenantId: "tenant-boom"}
		resp, err := server.RemoveOIDCMapping(ctx, req)
		require.Error(t, err)
		assert.NotNil(t, resp)
		assert.Contains(t, resp.GetMessage(), "delete failed")

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to remove trust")
	})
}

func TestBlockOIDCMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success blocks existing trust", func(t *testing.T) {
		existing := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Blocked:  new(false),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(mocktrust.WithTrust(existing))
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.BlockOIDCMappingRequest{TenantId: "tenant-123"}
		resp, err := server.BlockOIDCMapping(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("error maps to codes.Internal with message", func(t *testing.T) {
		internalErr := errors.New("database error")
		repo := mocktrust.NewInMemRepository(mocktrust.WithGetError(internalErr))
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.BlockOIDCMappingRequest{TenantId: "tenant-123"}
		resp, err := server.BlockOIDCMapping(ctx, req)
		require.Error(t, err)
		assert.NotNil(t, resp)
		assert.Contains(t, resp.GetMessage(), "database error")

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to block trust")
	})
}

func TestUnblockOIDCMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("success unblocks blocked trust", func(t *testing.T) {
		existing := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Blocked:  new(true),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(mocktrust.WithTrust(existing))
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.UnblockOIDCMappingRequest{TenantId: "tenant-123"}
		resp, err := server.UnblockOIDCMapping(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())
		assert.Empty(t, resp.GetMessage())
	})

	t.Run("error maps to codes.Internal with message", func(t *testing.T) {
		internalErr := errors.New("update failed")
		existing := trustv1.Trust_builder{
			TenantId: new("tenant-123"),
			Blocked:  new(true),
			Oidc: oidcv1.OIDC_builder{
				Issuer: new("https://issuer.example.com"),
			}.Build(),
		}.Build()
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithTrust(existing),
			mocktrust.WithUpdateError(internalErr),
		)
		svc := newTrust(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.UnblockOIDCMappingRequest{TenantId: "tenant-123"}
		resp, err := server.UnblockOIDCMapping(ctx, req)
		require.Error(t, err)
		assert.NotNil(t, resp)
		assert.Contains(t, resp.GetMessage(), "update failed")

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "failed to unblock trust")
	})
}
