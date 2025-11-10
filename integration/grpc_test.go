//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials/insecure"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	oidcproviderv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcprovider/v1"
	slogctx "github.com/veqryn/slog-context"
	stdgrpc "google.golang.org/grpc"

	"github.com/openkcm/session-manager/internal/dbtest/postgrestest"
	"github.com/openkcm/session-manager/internal/grpc"
	"github.com/openkcm/session-manager/internal/oidc"
	oidcsql "github.com/openkcm/session-manager/internal/oidc/sql"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

func TestGRPCServer(t *testing.T) {
	// given
	ctx := t.Context()
	port := 9091

	// create grpc server
	srv, service, terminateFn, err := startServer(t, port)
	require.NoError(t, err)
	defer srv.Stop()
	defer terminateFn(ctx)

	// grpc client connection
	conn, err := createClientConn(t, port)
	require.NoError(t, err)
	defer conn.Close()

	mappingClient := oidcmappingv1.NewServiceClient(conn)

	t.Run("BlockOIDCMapping", func(t *testing.T) {
		expJwks := []string{"jks"}
		expAud := []string{"aud"}
		expTenantID := uuid.NewString()
		expIssuer := uuid.NewString()
		applyResp, err := mappingClient.ApplyOIDCMapping(ctx, &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  expTenantID,
			Issuer:    expIssuer,
			JwksUris:  expJwks,
			Audiences: expAud,
		})
		assert.NoError(t, err)
		assert.True(t, applyResp.GetSuccess())

		blockResp, err := mappingClient.BlockOIDCMapping(ctx, &oidcmappingv1.BlockOIDCMappingRequest{
			TenantId: expTenantID,
		})
		assert.NoError(t, err)
		assert.True(t, blockResp.GetSuccess())

		actProvider, err := service.GetProvider(ctx, expIssuer)
		assert.NoError(t, err)
		assert.True(t, actProvider.Blocked)
		assert.Equal(t, expIssuer, actProvider.IssuerURL)
		assert.Equal(t, expAud, actProvider.Audiences)
		assert.Equal(t, expJwks, actProvider.JWKSURIs)
	})

	t.Run("UnblockOIDCMapping", func(t *testing.T) {
		expTenantID := uuid.NewString()
		expIssuer1 := uuid.NewString()
		applyRes, err := mappingClient.ApplyOIDCMapping(ctx, &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  expTenantID,
			Issuer:    expIssuer1,
			JwksUris:  []string{"uris"},
			Audiences: []string{"audience"},
		})
		assert.NoError(t, err)
		assert.True(t, applyRes.GetSuccess())

		blockRes, err := mappingClient.BlockOIDCMapping(ctx, &oidcmappingv1.BlockOIDCMappingRequest{
			TenantId: expTenantID,
		})
		assert.NoError(t, err)
		assert.True(t, blockRes.GetSuccess())

		actProvider, err := service.GetProvider(ctx, expIssuer1)
		assert.NoError(t, err)
		assert.True(t, actProvider.Blocked)

		unblockRes, err := mappingClient.UnblockOIDCMapping(ctx, &oidcmappingv1.UnblockOIDCMappingRequest{
			TenantId: expTenantID,
		})
		assert.NoError(t, err)
		assert.True(t, unblockRes.GetSuccess())

		actProvider, err = service.GetProvider(ctx, expIssuer1)
		assert.NoError(t, err)
		assert.False(t, actProvider.Blocked)
	})

	t.Run("RemoveOIDCMapping", func(t *testing.T) {
		expTenantID := uuid.NewString()
		expIssuer := uuid.NewString()
		applyRes, err := mappingClient.ApplyOIDCMapping(ctx, &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  expTenantID,
			Issuer:    expIssuer,
			JwksUris:  []string{"uris"},
			Audiences: []string{"audience"},
		})
		assert.NoError(t, err)
		assert.True(t, applyRes.GetSuccess())

		removeRes, err := mappingClient.RemoveOIDCMapping(ctx, &oidcmappingv1.RemoveOIDCMappingRequest{
			TenantId: expTenantID,
		})
		assert.NoError(t, err)
		assert.True(t, removeRes.GetSuccess())

		_, err = service.GetProvider(ctx, expIssuer)
		assert.ErrorIs(t, err, serviceerr.ErrNotFound)
	})
}

func createClientConn(t *testing.T, port int) (*stdgrpc.ClientConn, error) {
	t.Helper()
	conn, err := stdgrpc.NewClient(fmt.Sprintf("localhost:%d", port),
		stdgrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	return conn, err
}

func startServer(t *testing.T, port int) (*stdgrpc.Server, *oidc.Service, func(context.Context), error) {
	t.Helper()
	ctx := t.Context()
	// start postgres
	db, _, terminateFn := postgrestest.Start(ctx)
	service := oidc.NewService(oidcsql.NewRepository(db))

	lstConf := net.ListenConfig{}
	lis, err := lstConf.Listen(ctx, "tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, nil, nil, err
	}

	srv := stdgrpc.NewServer()
	oidcmappingv1.RegisterServiceServer(srv, grpc.NewOIDCMappingServer(service))
	oidcproviderv1.RegisterServiceServer(srv, grpc.NewOIDCProviderServer(service))

	// start
	go func() {
		err = srv.Serve(lis)
		slogctx.Error(ctx, "error while starting server", "error", err)
	}()

	return srv, service, terminateFn, nil
}
