package oidcmapping_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/runtime/protoimpl"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	flowv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/flow/v1"
	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	"github.com/openkcm/session-manager/modules/grpc/oidcmapping"
	"github.com/openkcm/session-manager/modules/oidctrust"
	mocktrust "github.com/openkcm/session-manager/modules/oidctrust/mocks"
	"github.com/openkcm/session-manager/pkg/serviceerr"
)

func TestNewOIDCMappingServer(t *testing.T) {
	repo := mocktrust.NewInMemRepository()
	svc := oidctrust.NewModule(repo)
	server := oidcmapping.NewServer(svc)
	assert.NotNil(t, server)
}

func TestApplyOIDCMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("forwards issuer, jwks_uri, audiences, client_id when set", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository()
		svc := oidctrust.NewModule(repo)
		server := oidcmapping.NewServer(svc)

		jwksURI := "https://issuer.example.com/.well-known/jwks.json"
		clientID := "client-abc"
		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  "tenant-123",
			Issuer:    "https://issuer.example.com",
			JwksUri:   &jwksURI,
			Audiences: []string{"audience1", "audience2"},
			ClientId:  clientID,
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
		svc := oidctrust.NewModule(repo)
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

	t.Run("forwards properties into all flow attribute extensions", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository()
		svc := oidctrust.NewModule(repo)
		server := oidcmapping.NewServer(svc)

		clientID := "client-xyz"
		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:   "tenant-with-props",
			Issuer:     "https://issuer.example.com",
			ClientId:   clientID,
			Properties: map[string]string{"foo": "bar", "baz": "qux"},
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())

		stored := repo.TGet("tenant-with-props")
		require.NotNil(t, stored)
		require.NotNil(t, stored.GetOidc())

		// Forwarding properties must not clobber the other request fields.
		assert.Equal(t, "tenant-with-props", stored.GetTenantId())
		assert.Equal(t, "https://issuer.example.com", stored.GetOidc().GetIssuer())
		assert.Equal(t, clientID, stored.GetOidc().GetClientId())

		// Every flow extension bucket must carry the full properties map as attributes.
		for _, ext := range []*protoimpl.ExtensionInfo{
			flowv1.E_AuthAttributes,
			flowv1.E_TokenAttributes,
			flowv1.E_LogoutAttributes,
			flowv1.E_AuthContext,
		} {
			attrs, ok := proto.GetExtension(stored.GetOidc(), ext).([]*flowv1.Attribute)
			require.Truef(t, ok, "extension %s should be []*flowv1.Attribute", ext.TypeDescriptor().FullName())

			got := make(map[string]string, len(attrs))
			for _, a := range attrs {
				got[a.GetKey()] = a.GetValue()
			}
			assert.Equalf(t, map[string]string{"foo": "bar", "baz": "qux"}, got,
				"extension %s should carry every property", ext.TypeDescriptor().FullName())
		}
	})

	t.Run("empty properties map leaves flow extensions unset", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository()
		svc := oidctrust.NewModule(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId: "tenant-no-props",
			Issuer:   "https://issuer.example.com",
		}

		resp, err := server.ApplyOIDCMapping(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())

		stored := repo.TGet("tenant-no-props")
		require.NotNil(t, stored)
		require.NotNil(t, stored.GetOidc())

		for _, ext := range []*protoimpl.ExtensionInfo{
			flowv1.E_AuthAttributes,
			flowv1.E_TokenAttributes,
			flowv1.E_LogoutAttributes,
			flowv1.E_AuthContext,
		} {
			assert.Falsef(t, proto.HasExtension(stored.GetOidc(), ext),
				"extension %s should be unset when no properties are provided", ext.TypeDescriptor().FullName())
		}
	})

	t.Run("ErrNotFound from Apply yields non-success response with message and no gRPC error", func(t *testing.T) {
		repo := mocktrust.NewInMemRepository(
			mocktrust.WithCreateError(serviceerr.ErrNotFound),
		)
		svc := oidctrust.NewModule(repo)
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
		svc := oidctrust.NewModule(repo)
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
		svc := oidctrust.NewModule(repo)
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
		svc := oidctrust.NewModule(repo)
		server := oidcmapping.NewServer(svc)

		req := &oidcmappingv1.RemoveOIDCMappingRequest{TenantId: "tenant-gone"}
		resp, err := server.RemoveOIDCMapping(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())
	})

	t.Run("other errors map to codes.Internal", func(t *testing.T) {
		deleteErr := errors.New("delete failed")
		repo := mocktrust.NewInMemRepository(mocktrust.WithDeleteError(deleteErr))
		svc := oidctrust.NewModule(repo)
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
		svc := oidctrust.NewModule(repo)
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
		svc := oidctrust.NewModule(repo)
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
		svc := oidctrust.NewModule(repo)
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
		svc := oidctrust.NewModule(repo)
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
