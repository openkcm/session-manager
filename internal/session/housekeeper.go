package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/zitadel/oidc/pkg/client/rp"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	slogctx "github.com/veqryn/slog-context"
)

func (m *Manager) TriggerHousekeeping(ctx context.Context, concurrencyLimit int, refreshTriggerInterval time.Duration) error {
	sessions, err := m.sessions.ListSessions(ctx)
	if err != nil {
		return err
	}
	slogctx.Info(ctx, "Start housekeeping sessions",
		"session_count", len(sessions),
		"concurrency_limit", concurrencyLimit,
		"token_refresh_trigger_interval", refreshTriggerInterval.String(),
	)

	// The following semaphore pattern limits the number of concurrent goroutines
	// to the specified concurrencyLimit. It follows Bryan C. Mills' famous talk
	// "Rethinking Classical Concurrency Patterns" at GopherCon 2018:
	// https://www.youtube.com/watch?v=5zXAHh5tJqQ

	// Define a token type for the semaphore
	type token struct{}

	// Create a buffered channel to act as the semaphore
	sem := make(chan token, concurrencyLimit)
	defer close(sem)

	// Start housekeeping sessions
	for _, s := range sessions {
		// Acquire a token before starting a new goroutine
		sem <- token{}
		go func(s Session) {
			m.housekeepSession(ctx, s, refreshTriggerInterval)
			// Release the token after the goroutine is done
			<-sem
		}(s)
	}

	// Wait for all goroutines to finish
	for n := concurrencyLimit; n > 0; n-- {
		sem <- token{}
	}

	return nil
}

func (m *Manager) housekeepSession(ctx context.Context, s Session, refreshTriggerInterval time.Duration) {
	// Create a short hash of the session ID for logging
	sessionIDHashBytes := sha256.Sum256([]byte(s.ID))
	sessionIDHash := hex.EncodeToString(sessionIDHashBytes[:])[:8]
	ctx = slogctx.With(ctx,
		"session_id_hash", sessionIDHash,
		"tenant_id", s.TenantID,
	)

	active, err := m.sessions.IsActive(ctx, s.ID)
	if err != nil {
		slogctx.Error(ctx, "Failed to get the session active status", "error", err)
		return
	}

	// Delete idle sessions if they have been idle for longer than the configured timeout
	if !active {
		err := m.sessions.DeleteSession(ctx, s)
		if err != nil {
			slogctx.Error(ctx, "Error deleting idle session", "error", err)
		} else {
			slogctx.Info(ctx, "Successfully deleted idle session")
		}
		return
	}

	// Refresh access tokens that are nearing expiration
	if time.Until(s.AccessTokenExpiry) < refreshTriggerInterval {
		err := m.refreshAccessToken(ctx, s)
		if err != nil {
			slogctx.Error(ctx, "Error refreshing access token", "error", err)
		} else {
			slogctx.Info(ctx, "Successfully refreshed access token")
		}
	}
}

func (m *Manager) relyingParty(oidc *oidcv1.OIDC) (rp.RelyingParty, error) {
	return m.cProvider.RelyingParty(oidc, m.callbackURL.String())
}

// refreshAccessToken refreshes the access token for the given session using its refresh token.
func (m *Manager) refreshAccessToken(ctx context.Context, s Session) error {
	trust, err := m.trust.Get(ctx, s.TenantID)
	if err != nil {
		return fmt.Errorf("could not get trust: %w", err)
	}

	oidc := trust.GetOidc()
	relyingParty, err := m.relyingParty(oidc)
	if err != nil {
		return fmt.Errorf("creating relying party: %w", err)
	}

	token, err := rp.RefreshAccessToken(relyingParty, s.RefreshToken, "", "")
	if err != nil {
		return fmt.Errorf("refreshing access token: %w", err)
	}

	s.AccessToken = token.AccessToken
	s.RefreshToken = token.RefreshToken
	s.AccessTokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	err = m.sessions.StoreSession(ctx, s)
	if err != nil {
		return fmt.Errorf("could not store refreshed session: %w", err)
	}

	return nil
}
