package session_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/session"
	sessionmock "github.com/openkcm/session-manager/internal/session/mock"
	"github.com/openkcm/session-manager/internal/trust"
	"github.com/openkcm/session-manager/internal/trust/trustmock"
)

func TestDeleteIdleSessions(t *testing.T) {
	// Arrange
	ctx := t.Context()
	sessionID := "test-session-id"
	cfg := &config.SessionManager{
		CSRFSecretParsed: []byte(testCSRFSecret),
	}
	sessions := sessionmock.NewInMemRepository(
		sessionmock.WithSession(session.Session{
			ID:                sessionID,
			TenantID:          "CMKTenantID",
			AccessTokenExpiry: time.Now().Add(2 * time.Hour),
		}),
	)

	err := sessions.BumpActive(ctx, sessionID, time.Hour)
	require.NoError(t, err)

	manager, err := session.NewManager(cfg, nil, sessions, nil, http.DefaultClient)
	require.NoError(t, err)

	// Session should be there before cleanup
	_, err = sessions.LoadSession(ctx, sessionID)
	require.NoError(t, err)

	// Perform cleanup with 1 hour idle duration
	err = manager.TriggerHousekeeping(ctx, 2, time.Hour)
	require.NoError(t, err)

	// Session should still be there after cleanup
	_, err = sessions.LoadSession(ctx, sessionID)
	require.NoError(t, err)

	// Now perform cleanup with 0 second idle duration
	err = sessions.BumpActive(ctx, sessionID, 0)
	require.NoError(t, err)

	err = manager.TriggerHousekeeping(ctx, 2, time.Hour)
	require.NoError(t, err)

	// Session should be deleted after cleanup
	_, err = sessions.LoadSession(ctx, sessionID)
	require.ErrorIs(t, err, serviceerr.ErrNotFound)
}

func TestRefreshAccessToken(t *testing.T) {
	ctx := t.Context()
	tenantID := "test-tenant"
	sessionID := "test-session-id"

	t.Run("Success - refreshes access token", func(t *testing.T) {
		// Setup mock OIDC discovery server first
		var tokenServerURL string
		var discoveryServerURL string
		discoveryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":         discoveryServerURL,
				"token_endpoint": tokenServerURL,
			})
		}))
		defer discoveryServer.Close()
		discoveryServerURL = discoveryServer.URL

		// Setup mock token server
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/token", r.URL.Path)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

			err := r.ParseForm()
			assert.NoError(t, err)
			assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
			assert.Equal(t, "old-refresh-token", r.Form.Get("refresh_token"))
			assert.Equal(t, "test-client-id", r.Form.Get("client_id"))
			assert.Equal(t, "param-value", r.Form.Get("test-param"))

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-access-token",
				"refresh_token": "new-refresh-token",
				"expires_in":    3600,
			})
		}))
		defer tokenServer.Close()
		tokenServerURL = tokenServer.URL + "/token"

		provider := trust.OIDCMapping{
			IssuerURL: discoveryServerURL,
			Properties: map[string]string{
				"test-param": "param-value",
			},
		}

		oidcRepo := trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, provider))

		sess := session.Session{
			ID:                sessionID,
			TenantID:          tenantID,
			RefreshToken:      "old-refresh-token",
			AccessToken:       "old-access-token",
			AccessTokenExpiry: time.Now().Add(30 * time.Second),
			Expiry:            time.Now().Add(1 * time.Hour),
		}

		sessions := sessionmock.NewInMemRepository(sessionmock.WithSession(sess))
		err := sessions.BumpActive(ctx, sessionID, time.Hour)
		require.NoError(t, err)

		cfg := &config.SessionManager{
			ClientAuth: config.ClientAuth{
				ClientID: "test-client-id",
			},
			AdditionalQueryParametersToken: []string{"test-param"},
			CSRFSecretParsed:               []byte(testCSRFSecret),
		}

		manager, err := session.NewManager(cfg, oidcRepo, sessions, nil, http.DefaultClient)
		require.NoError(t, err)

		// Trigger housekeeping which should refresh the token
		err = manager.TriggerHousekeeping(ctx, 1, 1*time.Minute)
		require.NoError(t, err)

		// Verify the session was updated with new tokens
		updatedSess, err := sessions.LoadSession(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", updatedSess.AccessToken)
		assert.Equal(t, "new-refresh-token", updatedSess.RefreshToken)
	})

	t.Run("Error - OIDC provider not found", func(t *testing.T) {
		oidcRepo := trustmock.NewInMemRepository()

		sess := session.Session{
			ID:                sessionID,
			TenantID:          tenantID,
			RefreshToken:      "old-refresh-token",
			AccessTokenExpiry: time.Now().Add(30 * time.Second),
			Expiry:            time.Now().Add(1 * time.Hour),
		}

		sessions := sessionmock.NewInMemRepository(sessionmock.WithSession(sess))
		err := sessions.BumpActive(ctx, sessionID, time.Hour)
		require.NoError(t, err)

		cfg := &config.SessionManager{
			ClientAuth: config.ClientAuth{
				ClientID: "test-client-id",
			},
			CSRFSecretParsed: []byte(testCSRFSecret),
		}

		manager, err := session.NewManager(cfg, oidcRepo, sessions, nil, http.DefaultClient)
		require.NoError(t, err)

		// Trigger housekeeping - should log error but not fail
		err = manager.TriggerHousekeeping(ctx, 1, 1*time.Minute)
		require.NoError(t, err)
	})

	t.Run("Error - token endpoint returns error", func(t *testing.T) {
		// Setup discovery server first
		var tokenServerURL string
		var discoveryServerURL string
		discoveryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":         discoveryServerURL,
				"token_endpoint": tokenServerURL,
			})
		}))
		defer discoveryServer.Close()
		discoveryServerURL = discoveryServer.URL

		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "invalid_grant"}`))
		}))
		defer tokenServer.Close()
		tokenServerURL = tokenServer.URL + "/token"

		provider := trust.OIDCMapping{
			IssuerURL:  discoveryServerURL,
			Properties: map[string]string{},
		}

		oidcRepo := trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, provider))

		sess := session.Session{
			ID:                sessionID,
			TenantID:          tenantID,
			RefreshToken:      "old-refresh-token",
			AccessToken:       "old-access-token",
			AccessTokenExpiry: time.Now().Add(30 * time.Second),
			Expiry:            time.Now().Add(1 * time.Hour),
		}

		sessions := sessionmock.NewInMemRepository(sessionmock.WithSession(sess))
		err := sessions.BumpActive(ctx, sessionID, time.Hour)
		require.NoError(t, err)

		cfg := &config.SessionManager{
			ClientAuth: config.ClientAuth{
				ClientID: "test-client-id",
			},
			CSRFSecretParsed: []byte(testCSRFSecret),
		}

		manager, err := session.NewManager(cfg, oidcRepo, sessions, nil, http.DefaultClient)
		require.NoError(t, err)

		// Trigger housekeeping - should log error but not fail
		err = manager.TriggerHousekeeping(ctx, 1, 1*time.Minute)
		require.NoError(t, err)

		// Token should not be updated
		updatedSess, err := sessions.LoadSession(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, "old-access-token", updatedSess.AccessToken)
	})

	t.Run("Error - missing token parameter property", func(t *testing.T) {
		var discoveryServerURL string
		discoveryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":         discoveryServerURL,
				"token_endpoint": discoveryServerURL + "/token",
			})
		}))
		defer discoveryServer.Close()
		discoveryServerURL = discoveryServer.URL

		provider := trust.OIDCMapping{
			IssuerURL:  discoveryServer.URL,
			Properties: map[string]string{}, // Missing required parameter
		}

		oidcRepo := trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, provider))

		sess := session.Session{
			ID:                sessionID,
			TenantID:          tenantID,
			RefreshToken:      "old-refresh-token",
			AccessTokenExpiry: time.Now().Add(30 * time.Second),
			Expiry:            time.Now().Add(1 * time.Hour),
		}

		sessions := sessionmock.NewInMemRepository(sessionmock.WithSession(sess))
		err := sessions.BumpActive(ctx, sessionID, time.Hour)
		require.NoError(t, err)

		cfg := &config.SessionManager{
			ClientAuth: config.ClientAuth{
				ClientID: "test-client-id",
			},
			AdditionalQueryParametersToken: []string{"missing-param"},
			CSRFSecretParsed:               []byte(testCSRFSecret),
		}

		manager, err := session.NewManager(cfg, oidcRepo, sessions, nil, http.DefaultClient)
		require.NoError(t, err)

		// Trigger housekeeping - should log error but not fail
		err = manager.TriggerHousekeeping(ctx, 1, 1*time.Minute)
		require.NoError(t, err)
	})
}

func TestHousekeepSession_ErrorCases(t *testing.T) {
	ctx := t.Context()

	t.Run("Session with no active status - gets deleted", func(t *testing.T) {
		sessionID := "test-session-id"

		sess := session.Session{
			ID:                sessionID,
			TenantID:          "test-tenant",
			AccessTokenExpiry: time.Now().Add(2 * time.Hour),
			Expiry:            time.Now().Add(2 * time.Hour),
		}

		sessions := sessionmock.NewInMemRepository(sessionmock.WithSession(sess))
		// Don't call BumpActive - session will not be active

		cfg := &config.SessionManager{
			CSRFSecretParsed: []byte(testCSRFSecret),
		}

		manager, err := session.NewManager(cfg, nil, sessions, nil, http.DefaultClient)
		require.NoError(t, err)

		err = manager.TriggerHousekeeping(ctx, 1, time.Hour)
		require.NoError(t, err)

		// Session should be deleted
		_, err = sessions.LoadSession(ctx, sessionID)
		require.ErrorIs(t, err, serviceerr.ErrNotFound)
	})

	t.Run("Session with active status but far from expiry - not refreshed", func(t *testing.T) {
		sessionID := "test-session-id"

		sess := session.Session{
			ID:                sessionID,
			TenantID:          "test-tenant",
			AccessToken:       "original-token",
			AccessTokenExpiry: time.Now().Add(2 * time.Hour), // Far from expiry
			Expiry:            time.Now().Add(2 * time.Hour),
		}

		sessions := sessionmock.NewInMemRepository(sessionmock.WithSession(sess))
		err := sessions.BumpActive(ctx, sessionID, time.Hour)
		require.NoError(t, err)

		cfg := &config.SessionManager{
			CSRFSecretParsed: []byte(testCSRFSecret),
		}

		manager, err := session.NewManager(cfg, nil, sessions, nil, http.DefaultClient)
		require.NoError(t, err)

		// Trigger with short refresh interval - token should not be refreshed
		err = manager.TriggerHousekeeping(ctx, 1, 30*time.Second)
		require.NoError(t, err)

		// Token should remain unchanged
		updatedSess, err := sessions.LoadSession(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, "original-token", updatedSess.AccessToken)
	})
}
