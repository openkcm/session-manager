package session_test

import (
	"context"
	"errors"
	"fmt"
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
		redirectURI = "http://sm.example.com/sm/callback"
		requestURI  = "http://cmk.example.com/ui"
		issuerURL   = "http://oidc.example.com"
		tenantID    = "tenant-id"
	)

	oidcProvider := oidc.Provider{
		IssuerURL: issuerURL,
		Blocked:   false,
		JWKSURIs:  []string{"http://jwks.example.com"},
		Audiences: []string{requestURI},
	}
	newOIDCRepo := func(getForTenantErr, createErr, deleteErr, updateErr error) *oidcmock.Repository {
		oidcRepo := oidcmock.NewInMemRepository(getForTenantErr, createErr, deleteErr, updateErr)
		_ = oidcRepo.Create(t.Context(), tenantID, oidcProvider)

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
			oidc:        newOIDCRepo(nil, nil, nil, nil),
			sessions:    sessionmock.NewInMemRepository(nil, nil, nil, nil),
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
			oidc:        newOIDCRepo(errors.New("faield to get oidc provider"), nil, nil, nil),
			sessions:    sessionmock.NewInMemRepository(nil, nil, nil, nil),
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
			oidc:        newOIDCRepo(nil, nil, nil, nil),
			sessions:    sessionmock.NewInMemRepository(nil, errors.New("failed to save state"), nil, nil),
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

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: "http://localhost:4043/logs"})
			require.NoError(t, err)

			m := session.NewManager(tt.oidc, tt.sessions, auditLogger, time.Hour, tt.redirectURI, tt.clientID)
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

func TestManager_RefreshTokens(t *testing.T) {
	type sessionFields struct {
		ID           string
		TenantID     string
		AccessToken  string
		RefreshToken string
		Expiry       time.Time
	}

	now := time.Now()
	aboutToExpire := now.Add(2 * time.Minute)
	fresh := now.Add(10 * time.Minute)

	makeSession := func(fields sessionFields) session.Session {
		return session.Session{
			ID:           fields.ID,
			TenantID:     fields.TenantID,
			AccessToken:  fields.AccessToken,
			RefreshToken: fields.RefreshToken,
			Expiry:       fields.Expiry,
		}
	}

	tests := []struct {
		name            string
		sessions        []session.Session
		refreshMock     func(ctx context.Context, refreshToken, clientID, tokenEndpoint string) (oidc.TokenResponse, error)
		storeSessionErr error
		wantRefreshed   bool
		wantErr         bool
	}{
		{
			name: "refreshes expiring session",
			sessions: []session.Session{
				makeSession(sessionFields{"s1", "tenant1", "old-token", "old-refresh", aboutToExpire}),
			},
			refreshMock: func(ctx context.Context, refreshToken, clientID, tokenEndpoint string) (oidc.TokenResponse, error) {
				return oidc.TokenResponse{
					AccessToken:  "new-token",
					RefreshToken: "new-refresh",
					ExpiresAt:    now.Add(1 * time.Hour),
				}, nil
			},
			wantRefreshed: true,
		},
		{
			name: "does not refresh fresh session",
			sessions: []session.Session{
				makeSession(sessionFields{"s2", "tenant2", "token", "refresh", fresh}),
			},
			refreshMock: func(ctx context.Context, refreshToken, clientID, tokenEndpoint string) (oidc.TokenResponse, error) {
				t.Fatalf("RefreshToken should not be called for fresh session")
				return oidc.TokenResponse{}, nil
			},
			wantRefreshed: false,
		},
		{
			name: "refresh error is handled",
			sessions: []session.Session{
				makeSession(sessionFields{"s3", "tenant3", "token", "refresh", aboutToExpire}),
			},
			refreshMock: func(ctx context.Context, refreshToken, clientID, tokenEndpoint string) (oidc.TokenResponse, error) {
				return oidc.TokenResponse{}, errors.New("refresh failed")
			},
			wantRefreshed: false,
		},
		{
			name: "store session error is handled",
			sessions: []session.Session{
				makeSession(sessionFields{"s4", "tenant4", "token", "refresh", aboutToExpire}),
			},
			refreshMock: func(ctx context.Context, refreshToken, clientID, tokenEndpoint string) (oidc.TokenResponse, error) {
				return oidc.TokenResponse{
					AccessToken:  "new-token",
					RefreshToken: "new-refresh",
					ExpiresAt:    now.Add(1 * time.Hour),
				}, nil
			},
			storeSessionErr: errors.New("store failed"),
			wantRefreshed:   false, // session is not updated in store
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOIDC := &oidcmock.Repository{
				ProvidersToTenant: map[string]oidc.Provider{
					"tenant1": {IssuerURL: "http://issuer1"},
					"tenant2": {IssuerURL: "http://issuer2"},
					"tenant3": {IssuerURL: "http://issuer3"},
					"tenant4": {IssuerURL: "http://issuer4"},
				},
			}
			sessionMap := map[string]session.Session{}
			for _, s := range tt.sessions {
				sessionMap[s.ID] = s
			}
			mockSessions := &sessionmock.Repository{
				Sessions:        sessionMap,
				StoreSessionErr: tt.storeSessionErr,
			}
			m := session.NewManager(mockOIDC, mockSessions, nil, time.Hour, "http://cb", "cid")

			err := m.RefreshTokens(t.Context())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			for _, s := range tt.sessions {
				stored, ok := mockSessions.Sessions[s.ID]
				if tt.wantRefreshed {
					assert.True(t, ok, "session should be stored")
					assert.Equal(t, "new-token", stored.AccessToken)
					assert.Equal(t, "new-refresh", stored.RefreshToken)
				} else {
					assert.Equal(t, s.AccessToken, stored.AccessToken)
					assert.Equal(t, s.RefreshToken, stored.RefreshToken)
				}
			}
		})
	}
}
