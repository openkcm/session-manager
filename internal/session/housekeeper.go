package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

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

// refreshAccessToken refreshes the access token for the given session using its refresh token.
func (m *Manager) refreshAccessToken(ctx context.Context, s Session) error {
	mapping, err := m.trustRepo.Get(ctx, s.TenantID)
	if err != nil {
		return fmt.Errorf("could not get trust mapping: %w", err)
	}

	openidConf, err := m.getOpenIDConfig(ctx, mapping.IssuerURL)
	if err != nil {
		return fmt.Errorf("could not get OpenID configuration: %w", err)
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", s.RefreshToken)
	data.Set("client_id", m.clientID)
	for _, parameter := range m.queryParametersToken {
		value, ok := mapping.Properties[parameter]
		if !ok {
			return fmt.Errorf("missing token parameter: %s", parameter)
		}
		data.Set(parameter, value)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openidConf.TokenEndpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.secureClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read token endpoint response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token endpoint returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	var respData tokenResponse
	err = json.Unmarshal(body, &respData)
	if err != nil {
		return fmt.Errorf("could not unmarshal token endpoint response: %w", err)
	}

	s.AccessToken = respData.AccessToken
	s.RefreshToken = respData.RefreshToken
	s.AccessTokenExpiry = time.Now().Add(time.Duration(respData.ExpiresIn))

	err = m.sessions.StoreSession(ctx, s)
	if err != nil {
		return fmt.Errorf("could not store refreshed session: %w", err)
	}

	return nil
}
