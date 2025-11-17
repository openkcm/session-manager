package session_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/oidc"
	oidcmock "github.com/openkcm/session-manager/internal/oidc/mock"
	"github.com/openkcm/session-manager/pkg/session"
	sessionmock "github.com/openkcm/session-manager/pkg/session/mock"
)

const (
	testCSRFSecret = "12345678901234567890123456789012" // NOSONAR
	testClientID   = "my-client-id"
)

func TestMakeRedirectURL(t *testing.T) {
	m, err := session.NewManager(&config.SessionManager{
		RedirectURL: "http://example.com/redirect",
		CSRFSecret:  commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
	}, nil, nil, nil, nil)
	require.NoError(t, err)

	tests := []struct {
		name       string
		requestURI string
		want       string
	}{
		{
			name:       "Basic redirect URL",
			requestURI: "http://example.com/request",
			want:       "http://example.com/redirect?to=http%3A%2F%2Fexample.com%2Frequest",
		}, {
			name:       "Different domain",
			requestURI: "http://ui.example.com/request",
			want:       "http://example.com/redirect?to=http%3A%2F%2Fui.example.com%2Frequest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.MakeRedirectURL(tt.requestURI)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestManager_Auth(t *testing.T) {
	const (
		requestURI  = "http://localhost/request.jwt"
		callbackURL = "http://localhost/sm/callback"
		redirectURL = "http://localhost/sm/redirect"
		tenantID    = "tenant-id"
	)

	oidcServer := StartOIDCServer(t, false)
	defer oidcServer.Close()

	auditServer := StartAuditServer(t)
	defer auditServer.Close()

	oidcProvider := oidc.Provider{
		IssuerURL: oidcServer.URL,
		Blocked:   false,
		JWKSURIs:  []string{"http://jwks.example.com"},
		Audiences: []string{requestURI},
		Properties: map[string]string{
			"paramAuth1":  "paramAuth1",
			"paramToken1": "paramToken1",
		},
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
		requestURI  string
		cfg         *config.SessionManager
		tenantID    string
		fingerprint string
		wantURL     string
		errAssert   assert.ErrorAssertionFunc
		provider    oidc.Provider
	}{
		{
			name:       "Success",
			oidc:       newOIDCRepo(nil, nil, nil, nil, nil),
			sessions:   sessionmock.NewInMemRepository(nil, nil, nil, nil, nil),
			requestURI: requestURI,
			cfg: &config.SessionManager{
				SessionDuration:                  time.Hour,
				CallbackURL:                      callbackURL,
				RedirectURL:                      redirectURL,
				AdditionalGetParametersAuthorize: []string{"paramAuth1"},
				JWSSigAlgs:                       []string{"RS256"},
				ClientAuth: config.ClientAuth{
					ClientID: testClientID,
				},
				CSRFSecret: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			tenantID:    tenantID,
			fingerprint: "fingerprint",
			wantURL:     oidcServer.URL + "/oauth2/authorize?client_id=my-client-id&code_challenge=someChallenge&code_challenge_method=S256&redirect_uri=" + callbackURL + "&response_type=code&scope=openid+profile+email+groups&state=someState&paramAuth1=paramAuth1",
			errAssert:   assert.NoError,
		},
		{
			name:       "Get OIDC error",
			oidc:       newOIDCRepo(nil, errors.New("faield to get oidc provider"), nil, nil, nil),
			sessions:   sessionmock.NewInMemRepository(nil, nil, nil, nil, nil),
			requestURI: requestURI,
			cfg: &config.SessionManager{
				SessionDuration: time.Hour,
				CallbackURL:     callbackURL,
				RedirectURL:     redirectURL,
				CSRFSecret:      commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			tenantID:    tenantID,
			fingerprint: "fingerprint",
			wantURL:     "",
			errAssert:   assert.Error,
		},
		{
			name:       "Save state error",
			oidc:       newOIDCRepo(nil, nil, nil, nil, nil),
			sessions:   sessionmock.NewInMemRepository(nil, errors.New("failed to save state"), nil, nil, nil),
			requestURI: requestURI,
			cfg: &config.SessionManager{
				SessionDuration: time.Hour,
				CallbackURL:     callbackURL,
				RedirectURL:     redirectURL,
				CSRFSecret:      commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			tenantID:    tenantID,
			fingerprint: "fingerprint",
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
				kParamAuth1          = "paramAuth1"
			)

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
			require.NoError(t, err)

			m, err := session.NewManager(tt.cfg, tt.oidc, tt.sessions, auditLogger, http.DefaultClient)
			require.NoError(t, err)
			got, err := m.MakeAuthURI(t.Context(), tt.tenantID, tt.fingerprint, tt.requestURI)

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
			assert.Equal(t, wantURL.Path, u.Path, "Path does not match")

			q := u.Query()
			wantQ := wantURL.Query()

			assert.Len(t, q, len(wantQ), "Query length does not match")

			assert.Equal(t, wantQ.Get(kResponseType), q.Get(kResponseType), "Unexpected response type")
			assert.Equal(t, wantQ.Get(kClientID), q.Get(kClientID), "Unexpected client id")
			assert.Equal(t, wantQ.Get(kCodeChallengeMethod), q.Get(kCodeChallengeMethod), "Unexpected code challenge")
			assert.Equal(t, wantQ.Get(kRedirectURI), q.Get(kRedirectURI), "Unexpected redirect URI")
			assert.Equal(t, wantQ.Get(kParamAuth1), q.Get(kParamAuth1), "Unexpected auth url")

			// Check the scopes on the URL string to ensure we don't have
			// something like scope=openid&scope=profile...
			// but rather scope=openid profile email groups
			scopeValues := url.Values{kScope: {"openid profile email groups"}}
			assert.Contains(t, got, scopeValues.Encode())

			// These values are generated randomly. So check if they aren't empty
			assert.NotEmpty(t, q.Get(kState), "State is zero")
			assert.NotEmpty(t, q.Get(kCodeChallenge), "Code challenge is zero")
		})
	}
}

func TestManager_FinaliseOIDCLogin(t *testing.T) {
	const (
		requestURI   = "http://cmk.example.com/ui"
		callbackURL  = "http://sm.example.com/sm/callback"
		redirectURL  = "http://sm.example.com/sm/redirect"
		tenantID     = "tenant-id"
		stateID      = "test-state-id"
		code         = "auth-code"
		fingerprint  = "test-fingerprint"
		pkceVerifier = "test-verifier"
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
		cfg             *config.SessionManager
		oidcServerFail  bool
		wantSessionID   bool
		wantCSRFToken   bool
		wantRedirectURI string
		errAssert       assert.ErrorAssertionFunc
	}{
		{
			name:        "Success",
			oidc:        oidcmock.NewInMemRepository(nil, nil, nil, nil, nil),
			sessions:    newSessionRepo(nil, nil, nil, nil, nil, &validState),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				SessionDuration:              time.Hour,
				CallbackURL:                  callbackURL,
				RedirectURL:                  redirectURL,
				AdditionalGetParametersToken: []string{"getParamToken1"},
				AdditionalAuthContextKeys:    []string{"authContextKey1"},
				JWSSigAlgs:                   []string{"RS256"},
				CSRFSecret:                   commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			wantSessionID:   true,
			wantCSRFToken:   true,
			wantRedirectURI: requestURI,
			errAssert:       assert.NoError,
		},
		{
			name:        "State load error",
			oidc:        oidcmock.NewInMemRepository(nil, nil, nil, nil, nil),
			sessions:    newSessionRepo(errors.New("state not found"), nil, nil, nil, nil, nil),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				SessionDuration: time.Hour,
				CSRFSecret:      commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "State expired",
			oidc:        oidcmock.NewInMemRepository(nil, nil, nil, nil, nil),
			sessions:    newSessionRepo(nil, nil, nil, nil, nil, &expiredState),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				JWSSigAlgs: []string{"RS256"},
				CSRFSecret: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "Fingerprint mismatch",
			oidc:        oidcmock.NewInMemRepository(nil, nil, nil, nil, nil),
			sessions:    newSessionRepo(nil, nil, nil, nil, nil, &mismatchState),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				JWSSigAlgs: []string{"RS256"},
				CSRFSecret: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "OIDC provider get error",
			oidc:        oidcmock.NewInMemRepository(nil, errors.New("provider not found"), nil, nil, nil),
			sessions:    newSessionRepo(nil, nil, nil, nil, nil, &validState),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				JWSSigAlgs: []string{"RS256"},
				CSRFSecret: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "Token exchange error",
			oidc:        oidcmock.NewInMemRepository(nil, nil, nil, nil, nil),
			sessions:    newSessionRepo(nil, nil, nil, nil, nil, &validState),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				JWSSigAlgs: []string{"RS256"},
				CSRFSecret: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			oidcServerFail:  true,
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "Auth context error",
			oidc:        oidcmock.NewInMemRepository(nil, nil, nil, nil, nil),
			sessions:    newSessionRepo(nil, nil, nil, nil, nil, &validState),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				SessionDuration:              time.Hour,
				CallbackURL:                  callbackURL,
				RedirectURL:                  redirectURL,
				AdditionalGetParametersToken: []string{"getParamToken1"},
				AdditionalAuthContextKeys:    []string{"doesNotExist"},
				JWSSigAlgs:                   []string{"RS256"},
				CSRFSecret:                   commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: testCSRFSecret},
			},
			wantSessionID:   true,
			wantCSRFToken:   true,
			wantRedirectURI: requestURI,
			errAssert:       assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oidcServer := StartOIDCServer(t, tt.oidcServerFail)
			defer oidcServer.Close()

			auditServer := StartAuditServer(t)
			defer auditServer.Close()

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
			require.NoError(t, err)

			jwksURI, err := url.JoinPath(oidcServer.URL, "/.well-known/jwks.json")
			require.NoError(t, err)

			localOIDCProvider := oidc.Provider{
				IssuerURL: oidcServer.URL,
				Blocked:   false,
				JWKSURIs:  []string{jwksURI},
				Audiences: []string{requestURI},
				Properties: map[string]string{
					"getParamToken1":  "getParamToken1",
					"authContextKey1": "authContextValue1",
				},
			}

			tt.oidc.Add(tenantID, localOIDCProvider)

			m, err := session.NewManager(tt.cfg, tt.oidc, tt.sessions, auditLogger, http.DefaultClient)
			require.NoError(t, err)

			result, err := m.FinaliseOIDCLogin(context.Background(), tt.stateID, tt.code, tt.fingerprint)

			if !tt.errAssert(t, err, fmt.Sprintf("Manager.Callback() error = %v", err)) {
				return
			}

			if err != nil {
				assert.Zero(t, result, "Result should be nil on error")
				return
			}

			require.NotNil(t, result, "Result should not be nil on success")

			sess := tt.sessions.Sessions[result.SessionID]
			assert.NotNil(t, sess.AuthContext)
			expKeys := []string{"issuer", "client_id"}
			for _, k := range expKeys {
				_, ok := sess.AuthContext[k]
				assert.True(t, ok)
			}

			if tt.wantSessionID {
				assert.NotEmpty(t, result.SessionID, "SessionID should not be empty")
			}

			if tt.wantCSRFToken {
				assert.NotEmpty(t, result.CSRFToken, "CSRFToken should not be empty")
			}

			if tt.wantRedirectURI != "" {
				assert.Equal(t, tt.wantRedirectURI, result.RequestURI, "RedirectURI should match")
			}
		})
	}
}
