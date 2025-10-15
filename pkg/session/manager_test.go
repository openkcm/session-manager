package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.com/openkcm/session-manager/internal/oidc"
	oidcmock "github.com/openkcm/session-manager/internal/oidc/mock"
	"github.com/openkcm/session-manager/pkg/session"
	sessionmock "github.com/openkcm/session-manager/pkg/session/mock"
)

func TestManager_Auth(t *testing.T) {
	const (
		redirectURI    = "http://sm.example.com/sm/callback"
		requestURI     = "http://cmk.example.com/ui"
		issuerURL      = "http://oidc.example.com"
		tenantID       = "tenant-id"
		testCSRFSecret = "12345678901234567890123456789012"
	)

	oidcProvider := oidc.Provider{
		IssuerURL: issuerURL,
		Blocked:   false,
		JWKSURIs:  []string{"http://jwks.example.com"},
		Audiences: []string{requestURI},
	}
	newOIDCRepo := func(getErr, getForTenantErr, createErr, deleteErr, updateErr error) *oidcmock.Repository {
		oidcRepo := oidcmock.NewInMemRepository(getErr, getForTenantErr, createErr, deleteErr, updateErr)
		oidcRepo.Add(tenantID, oidcProvider)

		return oidcRepo
	}

	tests := []struct {
		name        string
		oidc        *oidcmock.Repository
		sessions    *sessionmock.Repository
		redirectURI string
		clientID    string
		tenantID    string
		fingerprint string
		requestURI  string
		wantURL     string
		errAssert   assert.ErrorAssertionFunc
	}{
		{
			name:        "Success",
			oidc:        newOIDCRepo(nil, nil, nil, nil, nil),
			sessions:    sessionmock.NewInMemRepository(nil, nil, nil, nil, nil),
			redirectURI: redirectURI,
			clientID:    "my-client-id",
			tenantID:    tenantID,
			fingerprint: "fingerprint",
			requestURI:  requestURI,
			wantURL:     "http://oidc.example.com/?client_id=my-client-id&code_challenge=someChallenge&code_challenge_method=S256&redirect_uri=" + redirectURI + "&response_type=code&scope=openid&scope=profile&scope=email&scope=groups&state=someState",
			errAssert:   assert.NoError,
		},
		{
			name:        "Get OIDC error",
			oidc:        newOIDCRepo(nil, errors.New("faield to get oidc provider"), nil, nil, nil),
			sessions:    sessionmock.NewInMemRepository(nil, nil, nil, nil, nil),
			redirectURI: redirectURI,
			clientID:    "my-client-id",
			tenantID:    tenantID,
			fingerprint: "fingerprint",
			requestURI:  requestURI,
			wantURL:     "",
			errAssert:   assert.Error,
		},
		{
			name:        "Save state error",
			oidc:        newOIDCRepo(nil, nil, nil, nil, nil),
			sessions:    sessionmock.NewInMemRepository(nil, errors.New("failed to save state"), nil, nil, nil),
			redirectURI: redirectURI,
			clientID:    "my-client-id",
			tenantID:    tenantID,
			fingerprint: "fingerprint",
			requestURI:  requestURI,
			wantURL:     "",
			errAssert:   assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const (
				kScope               = "scope"
				kResponseType        = "response_type"
				kClientID            = "client_id"
				kState               = "state"
				kCodeChallenge       = "code_challenge"
				kCodeChallengeMethod = "code_challenge_method"
				kRedirectURI         = "redirect_uri"
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: server.URL})
			require.NoError(t, err)

			m := session.NewManager(tt.oidc, tt.sessions, auditLogger, time.Hour, tt.redirectURI, tt.clientID, testCSRFSecret)
			got, err := m.Auth(t.Context(), tt.tenantID, tt.fingerprint, tt.requestURI)
			t.Logf("Got Auth URL %s", got)

			if !tt.errAssert(t, err, fmt.Sprintf("Manager.Auth() error = %v", err)) || err != nil {
				return
			}

			// Validate that the data has been inserted into the repository
			assert.Equal(t, oidcProvider, tt.oidc.ProvidersToTenant[tt.tenantID], "OIDC Provider has not been inserted")

			// Check the returned URL
			u, err := url.Parse(got)
			require.NoError(t, err, "parsing location")

			wantURL, err := url.Parse(tt.wantURL)
			require.NoError(t, err, "parsing wanted URL")

			assert.Equal(t, wantURL.Hostname(), u.Hostname(), "Hostname does not match")

			q := u.Query()
			wantQ := wantURL.Query()

			assert.Len(t, q, len(wantQ), "Query length does not match")

			assert.Equal(t, wantQ.Get(kResponseType), q.Get(kResponseType), "Unexpected response type")
			assert.Equal(t, wantQ.Get(kClientID), q.Get(kClientID), "Unexpected client id")
			assert.Equal(t, wantQ.Get(kCodeChallengeMethod), q.Get(kCodeChallengeMethod), "Unexpected code challenge")
			assert.Equal(t, wantQ.Get(kRedirectURI), q.Get(kRedirectURI), "Unexpected redirect URI")

			// These values are generated randomly. So check if they aren't empty
			assert.NotEmpty(t, q.Get(kState), "State is zero")
			assert.NotEmpty(t, q.Get(kCodeChallenge), "Code challenge is zero")
		})
	}
}

func TestManager_Callback(t *testing.T) {
	const (
		redirectURI    = "http://sm.example.com/sm/callback"
		requestURI     = "http://cmk.example.com/ui"
		issuerURL      = "http://oidc.example.com"
		tenantID       = "tenant-id"
		stateID        = "test-state-id"
		code           = "auth-code"
		fingerprint    = "test-fingerprint"
		pkceVerifier   = "test-verifier"
		accessToken    = "access-token"
		refreshToken   = "refresh-token"
		testCSRFSecret = "12345678901234567890123456789012"
	)

	validState := session.State{
		ID:           stateID,
		TenantID:     tenantID,
		Fingerprint:  fingerprint,
		PKCEVerifier: pkceVerifier,
		RequestURI:   requestURI,
		Expiry:       time.Now().Add(time.Hour),
	}

	expiredState := session.State{
		ID:           stateID,
		TenantID:     tenantID,
		Fingerprint:  fingerprint,
		PKCEVerifier: pkceVerifier,
		RequestURI:   requestURI,
		Expiry:       time.Now().Add(-time.Hour),
	}

	mismatchState := session.State{
		ID:           stateID,
		TenantID:     tenantID,
		Fingerprint:  "different-fingerprint",
		PKCEVerifier: pkceVerifier,
		RequestURI:   requestURI,
		Expiry:       time.Now().Add(time.Hour),
	}

	newOIDCRepo := func(getErr, getForTenantErr, createErr, deleteErr, updateErr error) *oidcmock.Repository {
		oidcRepo := oidcmock.NewInMemRepository(getErr, getForTenantErr, createErr, deleteErr, updateErr)
		return oidcRepo
	}

	newSessionRepo := func(loadStateErr, storeStateErr, storeSessionErr, deleteStateErr, deleteSessionErr error, state *session.State) *sessionmock.Repository {
		sessionRepo := sessionmock.NewInMemRepository(loadStateErr, storeStateErr, storeSessionErr, deleteStateErr, deleteSessionErr)
		if storeSessionErr != nil {
			t.Logf("Session repo configured with store session error: %v", storeSessionErr)
		}
		if state != nil && loadStateErr == nil {
			if sessionRepo.States == nil {
				sessionRepo.States = make(map[string]session.State)
			}
			sessionRepo.States[state.ID] = *state
		}
		return sessionRepo
	}

	tests := []struct {
		name            string
		oidc            *oidcmock.Repository
		sessions        *sessionmock.Repository
		stateID         string
		code            string
		fingerprint     string
		wantSessionID   bool
		wantCSRFToken   bool
		wantRedirectURI string
		errAssert       assert.ErrorAssertionFunc
	}{
		{
			name:            "Success",
			oidc:            newOIDCRepo(nil, nil, nil, nil, nil),
			sessions:        newSessionRepo(nil, nil, nil, nil, nil, &validState),
			stateID:         stateID,
			code:            code,
			fingerprint:     fingerprint,
			wantSessionID:   true,
			wantCSRFToken:   true,
			wantRedirectURI: requestURI,
			errAssert:       assert.NoError,
		},
		{
			name:            "State load error",
			oidc:            newOIDCRepo(nil, nil, nil, nil, nil),
			sessions:        newSessionRepo(errors.New("state not found"), nil, nil, nil, nil, nil),
			stateID:         stateID,
			code:            code,
			fingerprint:     fingerprint,
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:            "State expired",
			oidc:            newOIDCRepo(nil, nil, nil, nil, nil),
			sessions:        newSessionRepo(nil, nil, nil, nil, nil, &expiredState),
			stateID:         stateID,
			code:            code,
			fingerprint:     fingerprint,
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:            "Fingerprint mismatch",
			oidc:            newOIDCRepo(nil, nil, nil, nil, nil),
			sessions:        newSessionRepo(nil, nil, nil, nil, nil, &mismatchState),
			stateID:         stateID,
			code:            code,
			fingerprint:     fingerprint,
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:            "OIDC provider get error",
			oidc:            newOIDCRepo(nil, errors.New("provider not found"), nil, nil, nil),
			sessions:        newSessionRepo(nil, nil, nil, nil, nil, &validState),
			stateID:         stateID,
			code:            code,
			fingerprint:     fingerprint,
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:            "Token exchange error",
			oidc:            newOIDCRepo(nil, nil, nil, nil, nil),
			sessions:        newSessionRepo(nil, nil, nil, nil, nil, &validState),
			stateID:         stateID,
			code:            code,
			fingerprint:     fingerprint,
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oidcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" && r.Method == http.MethodPost {
					if tt.name == "Token exchange error" {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"error": "invalid_request", "error_description": "Token exchange failed"}`))
						return
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					tokenResponse := map[string]interface{}{
						"access_token":  accessToken,
						"refresh_token": refreshToken,
						"id_token":      "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9...",
						"token_type":    "Bearer",
						"expires_in":    3600,
					}
					_ = json.NewEncoder(w).Encode(tokenResponse)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer oidcServer.Close()

			auditServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"success": true}`))
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer auditServer.Close()
			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
			require.NoError(t, err)

			oidcProviderWithMockServer := oidc.Provider{
				IssuerURL: oidcServer.URL,
				Blocked:   false,
				JWKSURIs:  []string{oidcServer.URL + "/jwks"},
				Audiences: []string{requestURI},
			}

			tt.oidc.Add(tenantID, oidcProviderWithMockServer)

			m := session.NewManager(tt.oidc, tt.sessions, auditLogger, time.Hour, redirectURI, "client-id", testCSRFSecret)

			result, err := m.Callback(context.Background(), tt.stateID, tt.code, tt.fingerprint)

			if !tt.errAssert(t, err, fmt.Sprintf("Manager.Callback() error = %v", err)) {
				return
			}

			if err != nil {
				assert.Nil(t, result, "Result should be nil on error")
				return
			}

			require.NotNil(t, result, "Result should not be nil on success")

			if tt.wantSessionID {
				assert.NotEmpty(t, result.SessionID, "SessionID should not be empty")
			}

			if tt.wantCSRFToken {
				assert.NotEmpty(t, result.CSRFToken, "CSRFToken should not be empty")
			}

			if tt.wantRedirectURI != "" {
				assert.Equal(t, tt.wantRedirectURI, result.RedirectURI, "RedirectURI should match")
			}
		})
	}
}
