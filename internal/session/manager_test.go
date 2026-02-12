package session_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/openid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/session"
	sessionmock "github.com/openkcm/session-manager/internal/session/mock"
	"github.com/openkcm/session-manager/internal/trust"
	oidcmock "github.com/openkcm/session-manager/internal/trust/mock"
)

const (
	testCSRFSecret = "12345678901234567890123456789012" // NOSONAR
	testClientID   = "my-client-id"
)

func TestManager_Auth(t *testing.T) {
	const (
		requestURI  = "http://localhost/request.jwt"
		callbackURL = "http://localhost/sm/callback"
		tenantID    = "tenant-id"
	)

	oidcServer := StartOIDCServer(t, false)
	defer oidcServer.Close()

	auditServer := StartAuditServer(t)
	defer auditServer.Close()

	oidcProvider := trust.Provider{
		IssuerURL: oidcServer.URL,
		Blocked:   false,
		JWKSURI:   "http://jwks.example.com",
		Audiences: []string{requestURI},
		Properties: map[string]string{
			"paramAuth1":  "paramAuth1",
			"paramToken1": "paramToken1",
		},
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
		provider    trust.Provider
	}{
		{
			name:       "Success",
			oidc:       oidcmock.NewInMemRepository(oidcmock.WithTrust(tenantID, oidcProvider)),
			sessions:   sessionmock.NewInMemRepository(),
			requestURI: requestURI,
			cfg: &config.SessionManager{
				SessionDuration:                    time.Hour,
				CallbackURL:                        callbackURL,
				AdditionalQueryParametersAuthorize: []string{"paramAuth1"},
				ClientAuth: config.ClientAuth{
					ClientID: testClientID,
				},
				CSRFSecretParsed: []byte(testCSRFSecret),
			},
			tenantID:    tenantID,
			fingerprint: "fingerprint",
			wantURL:     oidcServer.URL + "/oauth2/authorize?client_id=my-client-id&code_challenge=someChallenge&code_challenge_method=S256&redirect_uri=" + callbackURL + "&response_type=code&scope=openid+profile+email+groups&state=someState&paramAuth1=paramAuth1",
			errAssert:   assert.NoError,
		},
		{
			name: "Get OIDC error",
			oidc: oidcmock.NewInMemRepository(
				oidcmock.WithTrust(tenantID, oidcProvider),
				oidcmock.WithGetError(errors.New("faield to get oidc provider")),
			),
			sessions:   sessionmock.NewInMemRepository(),
			requestURI: requestURI,
			cfg: &config.SessionManager{
				SessionDuration:  time.Hour,
				CallbackURL:      callbackURL,
				CSRFSecretParsed: []byte(testCSRFSecret),
			},
			tenantID:    tenantID,
			fingerprint: "fingerprint",
			wantURL:     "",
			errAssert:   assert.Error,
		},
		{
			name:       "Save state error",
			oidc:       oidcmock.NewInMemRepository(oidcmock.WithTrust(tenantID, oidcProvider)),
			sessions:   sessionmock.NewInMemRepository(sessionmock.WithStoreStateError(errors.New("failed to save state"))),
			requestURI: requestURI,
			cfg: &config.SessionManager{
				SessionDuration:  time.Hour,
				CallbackURL:      callbackURL,
				CSRFSecretParsed: []byte(testCSRFSecret),
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
			assert.Equal(t, oidcProvider, tt.oidc.TGet(tt.tenantID), "OIDC Provider has not been inserted")

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
			oidc:        oidcmock.NewInMemRepository(),
			sessions:    sessionmock.NewInMemRepository(sessionmock.WithState(validState)),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				SessionDuration:                time.Hour,
				CallbackURL:                    callbackURL,
				AdditionalQueryParametersToken: []string{"queryParamToken1"},
				AdditionalAuthContextKeys:      []string{"authContextKey1"},
				CSRFSecretParsed:               []byte(testCSRFSecret),
			},
			wantSessionID:   true,
			wantCSRFToken:   true,
			wantRedirectURI: requestURI,
			errAssert:       assert.NoError,
		},
		{
			name:        "State load error",
			oidc:        oidcmock.NewInMemRepository(),
			sessions:    sessionmock.NewInMemRepository(sessionmock.WithLoadStateError(errors.New("state not found"))),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				SessionDuration:  time.Hour,
				CSRFSecretParsed: []byte(testCSRFSecret),
			},
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "State expired",
			oidc:        oidcmock.NewInMemRepository(),
			sessions:    sessionmock.NewInMemRepository(sessionmock.WithState(expiredState)),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
			},
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "Fingerprint mismatch",
			oidc:        oidcmock.NewInMemRepository(),
			sessions:    sessionmock.NewInMemRepository(sessionmock.WithState(mismatchState)),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
			},
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "OIDC provider get error",
			oidc:        oidcmock.NewInMemRepository(oidcmock.WithGetError(errors.New("provider not found"))),
			sessions:    sessionmock.NewInMemRepository(sessionmock.WithState(validState)),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
			},
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "Token exchange error",
			oidc:        oidcmock.NewInMemRepository(),
			sessions:    sessionmock.NewInMemRepository(sessionmock.WithState(validState)),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
			},
			oidcServerFail:  true,
			wantSessionID:   false,
			wantCSRFToken:   false,
			wantRedirectURI: "",
			errAssert:       assert.Error,
		},
		{
			name:        "Auth context error",
			oidc:        oidcmock.NewInMemRepository(),
			sessions:    sessionmock.NewInMemRepository(sessionmock.WithState(validState)),
			stateID:     stateID,
			code:        code,
			fingerprint: fingerprint,
			cfg: &config.SessionManager{
				SessionDuration:                time.Hour,
				CallbackURL:                    callbackURL,
				AdditionalQueryParametersToken: []string{"queryParamToken1"},
				AdditionalAuthContextKeys:      []string{"doesNotExist"},
				CSRFSecretParsed:               []byte(testCSRFSecret),
			},
			wantSessionID:   true,
			wantCSRFToken:   true,
			wantRedirectURI: requestURI,
			errAssert:       assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			oidcServer := StartOIDCServer(t, tt.oidcServerFail)
			defer oidcServer.Close()

			auditServer := StartAuditServer(t)
			defer auditServer.Close()

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
			require.NoError(t, err)

			jwksURI, err := url.JoinPath(oidcServer.URL, "/.well-known/jwks.json")
			require.NoError(t, err)

			localOIDCProvider := trust.Provider{
				IssuerURL: oidcServer.URL,
				Blocked:   false,
				JWKSURI:   jwksURI,
				Audiences: []string{requestURI},
				Properties: map[string]string{
					"queryParamToken1": "queryParamToken1",
					"authContextKey1":  "authContextValue1",
				},
			}

			tt.oidc.TAdd(tenantID, localOIDCProvider)

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

			sess, err := tt.sessions.LoadSession(ctx, result.SessionID)
			require.NoError(t, err, "Loading session from repository failed")
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

func TestManager_BCLogout(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       key,
		KeyID:     "kid1",
		Algorithm: string(jose.RS256),
	}}}
	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.RS256,
		Key:       jwks.Keys[0],
	}, nil)
	if err != nil {
		panic(err)
	}

	newJwt := func(claims any) string {
		token, err := jwt.Signed(signer).Claims(claims).Serialize()
		if err != nil {
			panic(err)
		}

		return token
	}

	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		publicJwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       &key.PublicKey,
			KeyID:     "kid1",
			Algorithm: string(jose.RS256),
		}}}
		b, err := json.Marshal(publicJwks)
		if err != nil {
			panic(err)
		}

		if _, err := w.Write(b); err != nil {
			panic(err)
		}
	}))

	tests := []struct {
		name      string
		cfg       *config.SessionManager
		jwt       string
		setupMock func(*oidcmock.Repository, *sessionmock.Repository)
		errAssert assert.ErrorAssertionFunc
	}{
		{
			name: "Success",
			cfg:  &config.SessionManager{},
			jwt: newJwt(struct {
				Events    map[string]struct{} `json:"events"`
				SessionID string              `json:"sid"`
				KeyID     string              `json:"kid"`
			}{
				Events:    map[string]struct{}{"http://schemas.openid.net/event/backchannel-logout": {}},
				SessionID: "sid-1",
			}),
			setupMock: func(oidcs *oidcmock.Repository, sessions *sessionmock.Repository) {
				_ = oidcs.Create(context.Background(), "tid-1", trust.Provider{
					IssuerURL: jwksSrv.URL,
				})
				_ = sessions.StoreSession(context.Background(), session.Session{ID: "sid-1", TenantID: "tid-1"})
			},
			errAssert: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			oidcServer := StartOIDCServer(t, false)
			defer oidcServer.Close()

			auditServer := StartAuditServer(t)
			defer auditServer.Close()

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
			require.NoError(t, err)

			oidcMock := oidcmock.NewInMemRepository()
			sessionMock := sessionmock.NewInMemRepository()

			cli := &http.Client{
				Transport: localRoundTripper{
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						b, err := json.Marshal(openid.Configuration{
							JwksURI: jwksSrv.URL,
							Issuer:  jwksSrv.URL,
						})
						if err != nil {
							panic(err)
						}

						if _, err := w.Write(b); err != nil {
							panic(err)
						}
					}),
				},
			}

			tt.setupMock(oidcMock, sessionMock)

			m, err := session.NewManager(tt.cfg, oidcMock, sessionMock, auditLogger, cli)
			require.NoError(t, err)

			err = m.BCLogout(ctx, tt.jwt)
			if !tt.errAssert(t, err, fmt.Sprintf("Manager.BCLogout() error = %v", err)) {
				return
			}
		})
	}
}

func TestManager_LogoutEdgeCases(t *testing.T) {
	const (
		tenantID  = "tenant-id"
		sessionID = "session-id"
	)

	tests := []struct {
		name      string
		sessionID string
		setupMock func(*oidcmock.Repository, *sessionmock.Repository)
		errAssert assert.ErrorAssertionFunc
	}{
		{
			name:      "Session not found",
			sessionID: "non-existent",
			setupMock: func(oidcs *oidcmock.Repository, sessions *sessionmock.Repository) {
				_ = sessions.StoreSession(context.Background(), session.Session{
					ID:       sessionID,
					TenantID: tenantID,
				})
			},
			errAssert: assert.Error,
		},
		{
			name:      "OIDC provider not found",
			sessionID: sessionID,
			setupMock: func(oidcs *oidcmock.Repository, sessions *sessionmock.Repository) {
				_ = sessions.StoreSession(context.Background(), session.Session{
					ID:       sessionID,
					TenantID: tenantID,
				})
			},
			errAssert: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			auditServer := StartAuditServer(t)
			defer auditServer.Close()

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
			require.NoError(t, err)

			oidcMock := oidcmock.NewInMemRepository()
			sessionMock := sessionmock.NewInMemRepository()

			tt.setupMock(oidcMock, sessionMock)

			cfg := &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				ClientAuth: config.ClientAuth{
					ClientID: testClientID,
				},
			}

			m, err := session.NewManager(cfg, oidcMock, sessionMock, auditLogger, http.DefaultClient)
			require.NoError(t, err)

			_, err = m.Logout(ctx, tt.sessionID)
			tt.errAssert(t, err)
		})
	}
}

func TestManager_BCLogout_ErrorCases(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       key,
		KeyID:     "kid1",
		Algorithm: string(jose.RS256),
	}}}
	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.RS256,
		Key:       jwks.Keys[0],
	}, nil)
	require.NoError(t, err)

	newJwt := func(claims any) string {
		token, err := jwt.Signed(signer).Claims(claims).Serialize()
		require.NoError(t, err)
		return token
	}

	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		publicJwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       &key.PublicKey,
			KeyID:     "kid1",
			Algorithm: string(jose.RS256),
		}}}
		b, err := json.Marshal(publicJwks)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(b)
	}))
	defer jwksSrv.Close()

	tests := []struct {
		name      string
		jwt       string
		setupMock func(*oidcmock.Repository, *sessionmock.Repository)
		errAssert assert.ErrorAssertionFunc
	}{
		{
			name: "Invalid JWT",
			jwt:  "invalid.jwt.token",
			setupMock: func(oidcs *oidcmock.Repository, sessions *sessionmock.Repository) {
			},
			errAssert: assert.Error,
		},
		{
			name: "Missing backchannel-logout event",
			jwt: newJwt(struct {
				Events    map[string]struct{} `json:"events"`
				SessionID string              `json:"sid"`
			}{
				Events:    map[string]struct{}{"http://invalid-event": {}},
				SessionID: "sid-1",
			}),
			setupMock: func(oidcs *oidcmock.Repository, sessions *sessionmock.Repository) {
			},
			errAssert: assert.Error,
		},
		{
			name: "Missing session ID",
			jwt: newJwt(struct {
				Events map[string]struct{} `json:"events"`
			}{
				Events: map[string]struct{}{"http://schemas.openid.net/event/backchannel-logout": {}},
			}),
			setupMock: func(oidcs *oidcmock.Repository, sessions *sessionmock.Repository) {
			},
			errAssert: assert.Error,
		},
		{
			name: "Session not found - should succeed",
			jwt: newJwt(struct {
				Events    map[string]struct{} `json:"events"`
				SessionID string              `json:"sid"`
			}{
				Events:    map[string]struct{}{"http://schemas.openid.net/event/backchannel-logout": {}},
				SessionID: "non-existent-session",
			}),
			setupMock: func(oidcs *oidcmock.Repository, sessions *sessionmock.Repository) {
			},
			errAssert: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			auditServer := StartAuditServer(t)
			defer auditServer.Close()

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
			require.NoError(t, err)

			oidcMock := oidcmock.NewInMemRepository()
			sessionMock := sessionmock.NewInMemRepository()

			cli := &http.Client{
				Transport: localRoundTripper{
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						b, _ := json.Marshal(openid.Configuration{
							JwksURI: jwksSrv.URL,
							Issuer:  jwksSrv.URL,
						})
						_, _ = w.Write(b)
					}),
				},
			}

			tt.setupMock(oidcMock, sessionMock)

			cfg := &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
			}

			m, err := session.NewManager(cfg, oidcMock, sessionMock, auditLogger, cli)
			require.NoError(t, err)

			err = m.BCLogout(ctx, tt.jwt)
			tt.errAssert(t, err)
		})
	}
}

func TestManager_NewManager_Error(t *testing.T) {
	auditServer := StartAuditServer(t)
	defer auditServer.Close()

	auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
	require.NoError(t, err)

	cfg := &config.SessionManager{
		CallbackURL:      "://invalid-url",
		CSRFSecretParsed: []byte(testCSRFSecret),
	}

	m, err := session.NewManager(cfg, oidcmock.NewInMemRepository(), sessionmock.NewInMemRepository(), auditLogger, http.DefaultClient)
	assert.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "parsing callback URL")
}

// localRoundTripper is an http.RoundTripper that executes HTTP transactions by
// using handler directly, instead of going over an HTTP connection.
type localRoundTripper struct {
	handler http.Handler
}

func (l localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	l.handler.ServeHTTP(w, req)
	return w.Result(), nil
}
