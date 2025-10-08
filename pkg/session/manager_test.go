package session_test

import (
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
			oidc:        newOIDCRepo(nil, errors.New("faield to get oidc provider"), nil, nil, nil),
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
			oidc:        newOIDCRepo(nil, nil, nil, nil, nil),
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
