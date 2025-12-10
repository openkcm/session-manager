package session_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/session"
	sessionmock "github.com/openkcm/session-manager/internal/session/mock"
)

func TestCleanupIdleSessions(t *testing.T) {
	// Arrange
	ctx := t.Context()
	sessionID := "test-session-id"
	cfg := &config.SessionManager{
		CSRFSecretParsed: []byte(testCSRFSecret),
	}
	sessions := sessionmock.NewInMemRepository(
		sessionmock.WithSession(session.Session{
			ID:          sessionID,
			TenantID:    "CMKTenantID",
			LastVisited: time.Now(),
		}),
	)
	manager, err := session.NewManager(cfg, nil, sessions, nil, http.DefaultClient)
	require.NoError(t, err)

	// Session should be there before cleanup
	_, err = sessions.LoadSession(ctx, sessionID)
	require.NoError(t, err)

	// Perform cleanup with 1 hour idle duration
	err = manager.CleanupIdleSessions(ctx, time.Hour)
	require.NoError(t, err)
	// Session should still be there after cleanup
	_, err = sessions.LoadSession(ctx, sessionID)
	require.NoError(t, err)

	// Now perform cleanup with 0 second idle duration
	err = manager.CleanupIdleSessions(ctx, 0)
	require.NoError(t, err)
	// Session should be deleted after cleanup
	_, err = sessions.LoadSession(ctx, sessionID)
	require.ErrorIs(t, err, serviceerr.ErrNotFound)
}
