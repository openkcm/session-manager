//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials/insecure"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	sessionv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/session/v1"
	slogctx "github.com/veqryn/slog-context"
	stdgrpc "google.golang.org/grpc"

	"github.com/openkcm/session-manager/internal/dbtest/postgrestest"
	"github.com/openkcm/session-manager/internal/grpc"
	"github.com/openkcm/session-manager/internal/oidc"
	oidcsql "github.com/openkcm/session-manager/internal/oidc/sql"
)

func TestGRPCServer(t *testing.T) {
	// given
	ctx := t.Context()
	port := 9091

	// create grpc server
	srv, _, terminateFn, err := startServer(t, port)
	require.NoError(t, err)
	defer srv.Stop()
	defer terminateFn(ctx)

	// grpc client connection
	conn, err := createClientConn(t, port)
	require.NoError(t, err)
	defer conn.Close()

	mappingClient := oidcmappingv1.NewServiceClient(conn)

	t.Run("ApplyOIDCMapping", func(t *testing.T) {
		expJwks := "jks"
		expTenantID := uuid.NewString()
		expIssuer := uuid.NewString()
		applyResp, err := mappingClient.ApplyOIDCMapping(ctx, &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  expTenantID,
			Issuer:    expIssuer,
			JwksUri:   &expJwks,
			Audiences: []string{"aud"},
		})
		assert.NoError(t, err)
		assert.True(t, applyResp.GetSuccess())
	})

	t.Run("BlockOIDCMapping", func(t *testing.T) {
		expJwks := "jks"
		expTenantID := uuid.NewString()
		expIssuer := uuid.NewString()
		applyResp, err := mappingClient.ApplyOIDCMapping(ctx, &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  expTenantID,
			Issuer:    expIssuer,
			JwksUri:   &expJwks,
			Audiences: []string{"aud"},
		})
		assert.NoError(t, err)
		assert.True(t, applyResp.GetSuccess())

		blockResp, err := mappingClient.BlockOIDCMapping(ctx, &oidcmappingv1.BlockOIDCMappingRequest{
			TenantId: expTenantID,
		})
		assert.NoError(t, err)
		assert.True(t, blockResp.GetSuccess())
	})

	t.Run("UnblockOIDCMapping", func(t *testing.T) {
		expJwks := "jks"
		expTenantID := uuid.NewString()
		expIssuer1 := uuid.NewString()
		applyRes, err := mappingClient.ApplyOIDCMapping(ctx, &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  expTenantID,
			Issuer:    expIssuer1,
			JwksUri:   &expJwks,
			Audiences: []string{"audience"},
		})
		assert.NoError(t, err)
		assert.True(t, applyRes.GetSuccess())

		blockRes, err := mappingClient.BlockOIDCMapping(ctx, &oidcmappingv1.BlockOIDCMappingRequest{
			TenantId: expTenantID,
		})
		assert.NoError(t, err)
		assert.True(t, blockRes.GetSuccess())

		unblockRes, err := mappingClient.UnblockOIDCMapping(ctx, &oidcmappingv1.UnblockOIDCMappingRequest{
			TenantId: expTenantID,
		})
		assert.NoError(t, err)
		assert.True(t, unblockRes.GetSuccess())
	})

	t.Run("RemoveOIDCMapping", func(t *testing.T) {
		expJwks := "jks"
		expTenantID := uuid.NewString()
		expIssuer := uuid.NewString()
		applyRes, err := mappingClient.ApplyOIDCMapping(ctx, &oidcmappingv1.ApplyOIDCMappingRequest{
			TenantId:  expTenantID,
			Issuer:    expIssuer,
			JwksUri:   &expJwks,
			Audiences: []string{"audience"},
		})
		assert.NoError(t, err)
		assert.True(t, applyRes.GetSuccess())

		removeRes, err := mappingClient.RemoveOIDCMapping(ctx, &oidcmappingv1.RemoveOIDCMappingRequest{
			TenantId: expTenantID,
		})
		assert.NoError(t, err)
		assert.True(t, removeRes.GetSuccess())
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
	oidcProviderRepo := oidcsql.NewRepository(db)
	service := oidc.NewService(oidcProviderRepo)

	lstConf := net.ListenConfig{}
	lis, err := lstConf.Listen(ctx, "tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, nil, nil, err
	}

	srv := stdgrpc.NewServer()
	oidcmappingv1.RegisterServiceServer(srv, grpc.NewOIDCMappingServer(service))
	sessionv1.RegisterServiceServer(srv, grpc.NewSessionServer(nil, oidcProviderRepo, http.DefaultClient, time.Hour))

	// start
	go func() {
		err = srv.Serve(lis)
		slogctx.Error(ctx, "error while starting server", "error", err)
	}()

	return srv, service, terminateFn, nil
}
