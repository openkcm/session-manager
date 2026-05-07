//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestGRPCServer(t *testing.T) {
	const cmdName = "api-server"
	const port = 9091

	// given
	ctx := t.Context()

	istat := initInfra(t)
	defer istat.Close(ctx)

	istat.Cfg.GRPC.Address = fmt.Sprintf(":%d", port)

	istat.PreparePostgres(t)
	istat.PrepareValKey(t)
	istat.PrepareConfig(t)

	currdir, err := os.Getwd()
	require.NoError(t, err, "failed to get wd")

	t.Chdir(istat.Procdir)

	commandCtx, cancelCommand := context.WithCancel(ctx)
	defer cancelCommand()

	cmd := exec.CommandContext(commandCtx, filepath.Join(currdir, "./session-manager"), cmdName)
	cmd.WaitDelay = 5 * time.Second
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }

	cmdOutPath := filepath.Join(currdir, "grpc.log")
	cmdOut, err := os.Create(cmdOutPath)
	if err != nil {
		t.Fatalf("failed to create an log file")
	}
	defer cmdOut.Close()

	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut

	t.Logf("starting an app process. Logs will be saved into %s", cmdOutPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start the server: %s", err)
	}

	errCh := make(chan error)
	go func() {
		if err := cmd.Wait(); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- fmt.Errorf("executing command: %w", err)
		}
		close(errCh)
	}()

	// grpc client connection
	cc, err := createClientConn(t, port)
	require.NoError(t, err)
	defer cc.Close()

	waitCtx, cancelWait := context.WithTimeout(commandCtx, 10*time.Second)
	defer cancelWait()
	if err := waitGRPCServerReady(waitCtx, cc); err != nil {
		t.Fatalf("waiting for the server readiness: %s", err)
	}

	trust := trustmappingv1.NewServiceClient(cc)

	t.Run("ApplyTrustMapping", func(t *testing.T) {
		expJwks := "jks"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer := uuid.Must(uuid.NewV4()).String()
		applyResp, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: []string{"aud"},
			}.Build(),
		}.Build())
		assert.NoError(t, err)
		assert.True(t, applyResp.GetSuccess())
	})

	t.Run("BlockTrustMapping", func(t *testing.T) {
		expJwks := "jks"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer := uuid.Must(uuid.NewV4()).String()
		applyResp, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: []string{"aud"},
			}.Build(),
		}.Build())
		assert.NoError(t, err)
		assert.True(t, applyResp.GetSuccess())

		blockResp, err := trust.BlockTrustMapping(ctx, trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())
		assert.NoError(t, err)
		assert.True(t, blockResp.GetSuccess())
	})

	t.Run("UnblockTrustMapping", func(t *testing.T) {
		expJwks := "jks"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer1 := uuid.Must(uuid.NewV4()).String()
		applyRes, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer1,
				JwksUri:   &expJwks,
				Audiences: []string{"audience"},
			}.Build(),
		}.Build())
		assert.NoError(t, err)
		assert.True(t, applyRes.GetSuccess())

		blockRes, err := trust.BlockTrustMapping(ctx, trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())
		assert.NoError(t, err)
		assert.True(t, blockRes.GetSuccess())

		unblockRes, err := trust.UnblockTrustMapping(ctx, trustmappingv1.UnblockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())
		assert.NoError(t, err)
		assert.True(t, unblockRes.GetSuccess())
	})

	t.Run("RemoveTrustMapping", func(t *testing.T) {
		expJwks := "jks"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer := uuid.Must(uuid.NewV4()).String()
		applyRes, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: []string{"audience"},
			}.Build(),
		}.Build())
		assert.NoError(t, err)
		assert.True(t, applyRes.GetSuccess())

		removeRes, err := trust.RemoveTrustMapping(ctx, trustmappingv1.RemoveTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())
		assert.NoError(t, err)
		assert.True(t, removeRes.GetSuccess())
	})

	t.Run("ApplyTrustMapping with multiple audiences", func(t *testing.T) {
		expJwks := "jks-multi"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer := uuid.Must(uuid.NewV4()).String()
		audiences := []string{"aud1", "aud2", "aud3"}

		applyResp, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: audiences,
			}.Build(),
		}.Build())
		assert.NoError(t, err)
		assert.True(t, applyResp.GetSuccess())
	})

	t.Run("ApplyTrustMapping idempotent - applying same mapping twice", func(t *testing.T) {
		expJwks := "jks-idempotent"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer := uuid.Must(uuid.NewV4()).String()

		// First application
		applyResp1, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: []string{"aud"},
			}.Build(),
		}.Build())

		assert.NoError(t, err)
		assert.True(t, applyResp1.GetSuccess())

		// Second application (should be idempotent)
		applyResp2, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: []string{"aud"},
			}.Build(),
		}.Build())

		assert.NoError(t, err)
		assert.True(t, applyResp2.GetSuccess())
	})

	t.Run("BlockTrustMapping idempotent - blocking twice", func(t *testing.T) {
		expJwks := "jks-block-twice"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer := uuid.Must(uuid.NewV4()).String()

		applyResp, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: []string{"aud"},
			}.Build(),
		}.Build())

		assert.NoError(t, err)
		assert.True(t, applyResp.GetSuccess())

		// First block
		blockResp1, err := trust.BlockTrustMapping(ctx, trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, blockResp1.GetSuccess())

		// Second block (should be idempotent)
		blockResp2, err := trust.BlockTrustMapping(ctx, trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, blockResp2.GetSuccess())
	})

	t.Run("UnblockTrustMapping idempotent - unblocking twice", func(t *testing.T) {
		expJwks := "jks-unblock-twice"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer := uuid.Must(uuid.NewV4()).String()

		applyRes, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: []string{"audience"},
			}.Build(),
		}.Build())

		assert.NoError(t, err)
		assert.True(t, applyRes.GetSuccess())

		blockRes, err := trust.BlockTrustMapping(ctx, trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, blockRes.GetSuccess())

		// First unblock
		unblockRes1, err := trust.UnblockTrustMapping(ctx, trustmappingv1.UnblockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, unblockRes1.GetSuccess())

		// Second unblock (should be idempotent)
		unblockRes2, err := trust.UnblockTrustMapping(ctx, trustmappingv1.UnblockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, unblockRes2.GetSuccess())
	})

	t.Run("Block and Unblock workflow", func(t *testing.T) {
		expJwks := "jks-workflow"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer := uuid.Must(uuid.NewV4()).String()

		// Apply mapping
		applyRes, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: []string{"audience"},
			}.Build(),
		}.Build())

		assert.NoError(t, err)
		assert.True(t, applyRes.GetSuccess())

		// Block it
		blockRes, err := trust.BlockTrustMapping(ctx, trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, blockRes.GetSuccess())

		// Unblock it
		unblockRes, err := trust.UnblockTrustMapping(ctx, trustmappingv1.UnblockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, unblockRes.GetSuccess())

		// Block again
		blockRes2, err := trust.BlockTrustMapping(ctx, trustmappingv1.BlockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, blockRes2.GetSuccess())

		// Unblock again
		unblockRes2, err := trust.UnblockTrustMapping(ctx, trustmappingv1.UnblockTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, unblockRes2.GetSuccess())
	})

	t.Run("RemoveTrustMapping idempotent - removing twice", func(t *testing.T) {
		expJwks := "jks-remove-twice"
		expTenantID := uuid.Must(uuid.NewV4()).String()
		expIssuer := uuid.Must(uuid.NewV4()).String()

		applyRes, err := trust.ApplyTrustMapping(ctx, trustmappingv1.ApplyTrustMappingRequest_builder{
			TenantId: &expTenantID,
			Oidc: trustmappingv1.ApplyTrustMappingRequest_ApplyOIDCTrust_builder{
				Issuer:    &expIssuer,
				JwksUri:   &expJwks,
				Audiences: []string{"audience"},
			}.Build(),
		}.Build())

		assert.NoError(t, err)
		assert.True(t, applyRes.GetSuccess())

		// First remove
		removeRes1, err := trust.RemoveTrustMapping(ctx, trustmappingv1.RemoveTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, removeRes1.GetSuccess())

		// Second remove - idempotence should not cause an error
		removeRes2, err := trust.RemoveTrustMapping(ctx, trustmappingv1.RemoveTrustMappingRequest_builder{
			TenantId: &expTenantID,
		}.Build())

		assert.NoError(t, err)
		assert.True(t, removeRes2.GetSuccess())
	})

	cancelCommand()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("error executing command: %s", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout exceeded")
	}
}

func createClientConn(t *testing.T, port int) (*grpc.ClientConn, error) {
	t.Helper()
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	return conn, err
}

func waitGRPCServerReady(ctx context.Context, cc *grpc.ClientConn) error {
	healthClient := healthpb.NewHealthClient(cc)

	const maxAttempts = 100
	for range maxAttempts {
		out, err := healthClient.Check(ctx, new(healthpb.HealthCheckRequest), grpc.WaitForReady(true))
		if err != nil {
			return fmt.Errorf("checking health status: %w", err)
		}

		if out.GetStatus() == healthpb.HealthCheckResponse_SERVING {
			return nil
		}
	}

	return errors.New("exceeded max attempts number")
}
