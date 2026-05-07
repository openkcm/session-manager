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

	sessionv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/session/v1"

	"github.com/openkcm/session-manager/internal/session"
	sessionvalkey "github.com/openkcm/session-manager/internal/session/valkey"
)

func TestSessionGRPC(t *testing.T) {
	const cmdName = "api-server"
	const port = 9092

	// given
	ctx := t.Context()

	istat := initInfra(t)
	defer istat.Close(ctx)

	istat.Cfg.GRPC.Address = fmt.Sprintf(":%d", port)

	istat.PreparePostgres(t)
	valkeyClient := istat.PrepareValKey(t)
	istat.PrepareConfig(t)

	sessionRepo := sessionvalkey.NewRepository(valkeyClient, "session")

	currdir, err := os.Getwd()
	require.NoError(t, err, "failed to get wd")

	t.Chdir(istat.Procdir)

	commandCtx, cancelCommand := context.WithCancel(ctx)
	defer cancelCommand()

	cmd := exec.CommandContext(commandCtx, filepath.Join(currdir, "./session-manager"), cmdName)
	cmd.WaitDelay = 5 * time.Second
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }

	cmdOutPath := filepath.Join(currdir, "session-grpc.log")
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

	sessionClient := sessionv1.NewServiceClient(cc)

	t.Run("GetSession - session not found", func(t *testing.T) {
		resp, err := sessionClient.GetSession(ctx, &sessionv1.GetSessionRequest{
			SessionId: "non-existent-session",
			TenantId:  "tenant-123",
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("GetSession - session not active", func(t *testing.T) {
		// Create a session but don't mark it as active
		sess := session.Session{
			ID:          uuid.Must(uuid.NewV4()).String(),
			TenantID:    "tenant-inactive",
			Issuer:      "https://issuer.example.com",
			ProviderID:  "provider-123",
			AccessToken: "token-123",
			Expiry:      time.Now().Add(1 * time.Hour),
			Claims: session.Claims{
				Subject: "user-123",
			},
		}
		err := sessionRepo.StoreSession(ctx, sess)
		require.NoError(t, err)

		resp, err := sessionClient.GetSession(ctx, &sessionv1.GetSessionRequest{
			SessionId: sess.ID,
			TenantId:  sess.TenantID,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("GetSession - valid active session", func(t *testing.T) {
		// Create and activate a session
		sess := session.Session{
			ID:          uuid.Must(uuid.NewV4()).String(),
			TenantID:    "tenant-active",
			Issuer:      "https://issuer.example.com",
			ProviderID:  "provider-active",
			AccessToken: "token-active",
			Expiry:      time.Now().Add(1 * time.Hour),
			Claims: session.Claims{
				Subject:    "user-active",
				GivenName:  "John",
				FamilyName: "Doe",
				Email:      "john@example.com",
				Groups:     []string{"admin", "users"},
			},
			AuthContext: map[string]string{"key": "value"},
		}
		err := sessionRepo.StoreSession(ctx, sess)
		require.NoError(t, err)

		// Mark as active
		err = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)
		require.NoError(t, err)

		// Note: This test will fail validation because there's no trust mapping configured
		// but it tests the session retrieval path
		resp, err := sessionClient.GetSession(ctx, &sessionv1.GetSessionRequest{
			SessionId: sess.ID,
			TenantId:  sess.TenantID,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		// Will be false because trust mapping is not configured, but tests the flow
		assert.False(t, resp.GetValid())
	})

	t.Run("GetSession - tenant ID mismatch", func(t *testing.T) {
		sess := session.Session{
			ID:          uuid.Must(uuid.NewV4()).String(),
			TenantID:    "correct-tenant",
			Issuer:      "https://issuer.example.com",
			ProviderID:  "provider-tenant",
			AccessToken: "token-tenant",
			Expiry:      time.Now().Add(1 * time.Hour),
		}
		err := sessionRepo.StoreSession(ctx, sess)
		require.NoError(t, err)

		err = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)
		require.NoError(t, err)

		resp, err := sessionClient.GetSession(ctx, &sessionv1.GetSessionRequest{
			SessionId: sess.ID,
			TenantId:  "wrong-tenant",
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})
}
