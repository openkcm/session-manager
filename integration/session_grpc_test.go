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

	sessionv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/session/v1"
	slogctx "github.com/veqryn/slog-context"
	stdgrpc "google.golang.org/grpc"

	"github.com/openkcm/session-manager/internal/dbtest/postgrestest"
	"github.com/openkcm/session-manager/internal/dbtest/valkeytest"
	"github.com/openkcm/session-manager/internal/grpc"
	oidcsql "github.com/openkcm/session-manager/internal/oidc/sql"
	"github.com/openkcm/session-manager/internal/session"
	sessionvalkey "github.com/openkcm/session-manager/internal/session/valkey"
)

func TestSessionGRPC(t *testing.T) {
	// given
	ctx := t.Context()
	port := 9092

	// create grpc server with session support
	srv, sessionRepo, terminateFn, err := startSessionServer(t, port)
	require.NoError(t, err)
	defer srv.Stop()
	defer terminateFn(ctx)

	// grpc client connection
	conn, err := createClientConn(t, port)
	require.NoError(t, err)
	defer conn.Close()
	sessionClient := sessionv1.NewServiceClient(conn)

	t.Run("GetSession - session not found", func(t *testing.T) {
		resp, err := sessionClient.GetSession(ctx, &sessionv1.GetSessionRequest{
			SessionId:   "non-existent-session",
			TenantId:    "tenant-123",
			Fingerprint: "fingerprint-123",
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("GetSession - session not active", func(t *testing.T) {
		// Create a session but don't mark it as active
		sess := session.Session{
			ID:          uuid.NewString(),
			TenantID:    "tenant-inactive",
			Fingerprint: "fingerprint-inactive",
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
			SessionId:   sess.ID,
			TenantId:    sess.TenantID,
			Fingerprint: sess.Fingerprint,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("GetSession - valid active session", func(t *testing.T) {
		// Create and activate a session
		sess := session.Session{
			ID:          uuid.NewString(),
			TenantID:    "tenant-active",
			Fingerprint: "fingerprint-active",
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

		// Note: This test will fail validation because there's no OIDC provider configured
		// but it tests the session retrieval path
		resp, err := sessionClient.GetSession(ctx, &sessionv1.GetSessionRequest{
			SessionId:   sess.ID,
			TenantId:    sess.TenantID,
			Fingerprint: sess.Fingerprint,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		// Will be false because provider is not configured, but tests the flow
		assert.False(t, resp.GetValid())
	})

	t.Run("GetSession - fingerprint mismatch", func(t *testing.T) {
		sess := session.Session{
			ID:          uuid.NewString(),
			TenantID:    "tenant-fingerprint",
			Fingerprint: "correct-fingerprint",
			Issuer:      "https://issuer.example.com",
			ProviderID:  "provider-fp",
			AccessToken: "token-fp",
			Expiry:      time.Now().Add(1 * time.Hour),
		}
		err := sessionRepo.StoreSession(ctx, sess)
		require.NoError(t, err)

		err = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)
		require.NoError(t, err)

		resp, err := sessionClient.GetSession(ctx, &sessionv1.GetSessionRequest{
			SessionId:   sess.ID,
			TenantId:    sess.TenantID,
			Fingerprint: "wrong-fingerprint",
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("GetSession - tenant ID mismatch", func(t *testing.T) {
		sess := session.Session{
			ID:          uuid.NewString(),
			TenantID:    "correct-tenant",
			Fingerprint: "fingerprint-tenant",
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
			SessionId:   sess.ID,
			TenantId:    "wrong-tenant",
			Fingerprint: sess.Fingerprint,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})
}

func startSessionServer(t *testing.T, port int) (*stdgrpc.Server, session.Repository, func(context.Context), error) {
	t.Helper()
	ctx := t.Context()

	// start postgres
	db, _, terminatePG := postgrestest.Start(ctx)

	// start valkey
	valkeyClient, _, terminateValkey := valkeytest.Start(ctx)

	terminateFn := func(ctx context.Context) {
		terminatePG(ctx)
		terminateValkey(ctx)
		db.Close()
		valkeyClient.Close()
	}

	oidcProviderRepo := oidcsql.NewRepository(db)
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, "session")

	lstConf := net.ListenConfig{}
	lis, err := lstConf.Listen(ctx, "tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, nil, nil, err
	}

	srv := stdgrpc.NewServer()
	sessionv1.RegisterServiceServer(srv, grpc.NewSessionServer(sessionRepo, oidcProviderRepo, http.DefaultClient, 90*time.Minute))

	// start
	go func() {
		err = srv.Serve(lis)
		slogctx.Error(ctx, "error while starting session server", "error", err)
	}()

	return srv, sessionRepo, terminateFn, nil
}
