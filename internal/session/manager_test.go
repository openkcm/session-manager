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
	"github.com/openkcm/common-sdk/pkg/oidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
	"github.com/openkcm/session-manager/internal/session"
	sessionmock "github.com/openkcm/session-manager/internal/session/mock"
	"github.com/openkcm/session-manager/internal/trust"
	"github.com/openkcm/session-manager/internal/trust/trustmock"
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

	oidcMapping := trust.OIDCMapping{
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
		oidc        *trustmock.Repository
		sessions    *sessionmock.Repository
		requestURI  string
		cfg         *config.SessionManager
		tenantID    string
		fingerprint string
		wantURL     string
		errAssert   assert.ErrorAssertionFunc
		mapping     trust.OIDCMapping
	}{
		{
			name:       "Success",
			oidc:       trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, oidcMapping)),
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
			name: "Get trust mapping error",
			oidc: trustmock.NewInMemRepository(
				trustmock.WithTrust(tenantID, oidcMapping),
				trustmock.WithGetError(errors.New("failed to get trust mapping")),
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
			oidc:       trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, oidcMapping)),
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

			m, err := session.NewManager(
				tt.cfg,
				tt.oidc,
				tt.sessions,
				auditLogger,
				session.WithAllowHttpScheme(true),
			)
			require.NoError(t, err)
			got, _, err := m.MakeAuthURI(t.Context(), tt.tenantID, tt.fingerprint, tt.requestURI)

			if !tt.errAssert(t, err, fmt.Sprintf("Manager.Auth() error = %v", err)) || err != nil {
				return
			}

			assert.Equal(t, oidcMapping, tt.oidc.TGet(tt.tenantID), "Trust mapping has not been inserted")

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

			scopeValues := url.Values{kScope: {"openid profile email groups"}}
			assert.Contains(t, got, scopeValues.Encode())

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
		oidc            *trustmock.Repository
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
			oidc:        trustmock.NewInMemRepository(),
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
			oidc:        trustmock.NewInMemRepository(),
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
			oidc:        trustmock.NewInMemRepository(),
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
			oidc:        trustmock.NewInMemRepository(),
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
			name:        "Trust mapping get error",
			oidc:        trustmock.NewInMemRepository(trustmock.WithGetError(errors.New("trust mapping not found"))),
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
			oidc:        trustmock.NewInMemRepository(),
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

			localOIDCMapping := trust.OIDCMapping{
				IssuerURL: oidcServer.URL,
				Blocked:   false,
				JWKSURI:   jwksURI,
				Audiences: []string{requestURI},
				Properties: map[string]string{
					"queryParamToken1": "queryParamToken1",
					"authContextKey1":  "authContextValue1",
				},
			}

			tt.oidc.TAdd(tenantID, localOIDCMapping)

			m, err := session.NewManager(
				tt.cfg,
				tt.oidc,
				tt.sessions,
				auditLogger,
				session.WithAllowHttpScheme(true),
			)
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

	const keyID = "kid1"
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       key,
		KeyID:     keyID,
		Algorithm: string(jose.RS256),
	}}}
	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.RS256,
		Key:       jwks.Keys[0],
	}, (&jose.SignerOptions{}).WithType("JWT"))
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

	publicJwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &key.PublicKey,
		KeyID:     keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}}

	var jwksSrv *httptest.Server
	jwksSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(oidc.Configuration{
				Issuer:  jwksSrv.URL,
				JwksURI: jwksSrv.URL + "/jwks",
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(publicJwks)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jwksSrv.Close()

	tests := []struct {
		name      string
		cfg       *config.SessionManager
		jwt       string
		setupMock func(*trustmock.Repository, *sessionmock.Repository)
		errAssert assert.ErrorAssertionFunc
	}{
		{
			name: "Success",
			cfg:  &config.SessionManager{},
			setupMock: func(oidcs *trustmock.Repository, sessions *sessionmock.Repository) {
				_ = oidcs.Create(context.Background(), "tid-1", trust.OIDCMapping{
					IssuerURL: jwksSrv.URL,
				})
				_ = sessions.StoreSession(context.Background(), session.Session{
					ID:         "session-1",
					ProviderID: "sid-1",
					TenantID:   "tid-1",
				})
			},
			errAssert: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			auditServer := StartAuditServer(t)
			defer auditServer.Close()

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
			require.NoError(t, err)

			oidcMock := trustmock.NewInMemRepository()
			sessionMock := sessionmock.NewInMemRepository()

			tt.setupMock(oidcMock, sessionMock)

			now := time.Now()
			logoutJWT := newJwt(map[string]any{
				"iss":    jwksSrv.URL,
				"sub":    "user-1",
				"aud":    []string{""},
				"exp":    now.Add(time.Hour).Unix(),
				"iat":    now.Unix(),
				"sid":    "sid-1",
				"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}},
			})

			if tt.jwt == "" {
				tt.jwt = logoutJWT
			}

			m, err := session.NewManager(
				tt.cfg,
				oidcMock,
				sessionMock,
				auditLogger,
				session.WithAllowHttpScheme(true),
			)
			require.NoError(t, err)

			err = m.BCLogout(ctx, tt.jwt)
			if !tt.errAssert(t, err, fmt.Sprintf("Manager.BCLogout() error = %v", err)) {
				return
			}

			_, loadErr := sessionMock.LoadSession(ctx, "session-1")
			assert.Error(t, loadErr, "session should have been deleted after BCLogout")
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
		setupMock func(*trustmock.Repository, *sessionmock.Repository)
		errAssert assert.ErrorAssertionFunc
	}{
		{
			name:      "Session not found",
			sessionID: "non-existent",
			setupMock: func(oidcs *trustmock.Repository, sessions *sessionmock.Repository) {
				_ = sessions.StoreSession(context.Background(), session.Session{
					ID:       sessionID,
					TenantID: tenantID,
				})
			},
			errAssert: assert.Error,
		},
		{
			name:      "Trust mapping not found",
			sessionID: sessionID,
			setupMock: func(oidcs *trustmock.Repository, sessions *sessionmock.Repository) {
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

			oidcMock := trustmock.NewInMemRepository()
			sessionMock := sessionmock.NewInMemRepository()

			tt.setupMock(oidcMock, sessionMock)

			cfg := &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				ClientAuth: config.ClientAuth{
					ClientID: testClientID,
				},
			}

			m, err := session.NewManager(cfg, oidcMock, sessionMock, auditLogger)
			require.NoError(t, err)

			_, err = m.Logout(ctx, tt.sessionID)
			tt.errAssert(t, err)
		})
	}
}

func TestManager_Logout_RedirectURL(t *testing.T) {
	const (
		tenantID  = "tenant-id"
		sessionID = "session-id"
	)

	newOIDCDiscoveryServer := func(endSessionEndpoint string) *httptest.Server {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(oidc.Configuration{
				Issuer:             srv.URL,
				JwksURI:            srv.URL + "/jwks",
				EndSessionEndpoint: endSessionEndpoint,
			})
		}))
		return srv
	}

	tests := []struct {
		name              string
		endSessionURL     string
		postLogoutURL     string
		queryParamsLogout []string
		mappingProps      map[string]string
		deleteSessionErr  error
		wantURL           string
		wantContains      []string
		errAssert         assert.ErrorAssertionFunc
		errContains       string
	}{
		{
			name:          "Success with end session endpoint and post logout redirect",
			endSessionURL: "https://idp.example.com/logout",
			postLogoutURL: "https://app.example.com/landing",
			wantContains: []string{
				"https://idp.example.com/logout",
				"client_id=" + testClientID,
				"post_logout_redirect_uri=" + url.QueryEscape("https://app.example.com/landing"),
			},
			errAssert: assert.NoError,
		},
		{
			name:          "Success with end session endpoint without post logout redirect",
			endSessionURL: "https://idp.example.com/logout",
			postLogoutURL: "",
			wantContains: []string{
				"https://idp.example.com/logout",
				"client_id=" + testClientID,
			},
			errAssert: assert.NoError,
		},
		{
			name:              "Success with additional logout query parameters",
			endSessionURL:     "https://idp.example.com/logout",
			postLogoutURL:     "",
			queryParamsLogout: []string{"logoutParam1"},
			mappingProps:      map[string]string{"logoutParam1": "logoutValue1"},
			wantContains: []string{
				"client_id=" + testClientID,
				"logoutParam1=logoutValue1",
			},
			errAssert: assert.NoError,
		},
		{
			name:          "No end session endpoint with post logout redirect URL",
			endSessionURL: "",
			postLogoutURL: "https://app.example.com/landing",
			wantURL:       "https://app.example.com/landing",
			errAssert:     assert.NoError,
		},
		{
			name:          "No end session endpoint and no post logout redirect URL",
			endSessionURL: "",
			postLogoutURL: "",
			errAssert:     assert.Error,
			errContains:   "end_session_not_supported",
		},
		{
			name:              "Missing logout query parameter in mapping properties",
			endSessionURL:     "https://idp.example.com/logout",
			postLogoutURL:     "",
			queryParamsLogout: []string{"missingParam"},
			mappingProps:      map[string]string{},
			wantContains: []string{
				"client_id=" + testClientID,
			},
			errAssert: assert.NoError,
		},
		{
			name:             "Delete session error",
			endSessionURL:    "https://idp.example.com/logout",
			deleteSessionErr: errors.New("storage failure"),
			errAssert:        assert.Error,
			errContains:      "deleting session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			oidcSrv := newOIDCDiscoveryServer(tt.endSessionURL)
			defer oidcSrv.Close()

			auditServer := StartAuditServer(t)
			defer auditServer.Close()

			auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
			require.NoError(t, err)

			props := tt.mappingProps
			if props == nil {
				props = map[string]string{}
			}

			oidcMock := trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, trust.OIDCMapping{
				IssuerURL:  oidcSrv.URL,
				Properties: props,
			}))

			var sessionOpts []sessionmock.RepositoryOption
			sessionOpts = append(sessionOpts, sessionmock.WithSession(session.Session{
				ID:       sessionID,
				TenantID: tenantID,
			}))
			if tt.deleteSessionErr != nil {
				sessionOpts = append(sessionOpts, sessionmock.WithDeleteSessionError(tt.deleteSessionErr))
			}
			sessMock := sessionmock.NewInMemRepository(sessionOpts...)

			cfg := &config.SessionManager{
				CSRFSecretParsed:                []byte(testCSRFSecret),
				PostLogoutRedirectURL:           tt.postLogoutURL,
				AdditionalQueryParametersLogout: tt.queryParamsLogout,
				ClientAuth: config.ClientAuth{
					ClientID: testClientID,
				},
			}

			m, err := session.NewManager(cfg, oidcMock, sessMock, auditLogger, session.WithAllowHttpScheme(true))
			require.NoError(t, err)

			got, err := m.Logout(ctx, sessionID)

			if !tt.errAssert(t, err, fmt.Sprintf("Manager.Logout() error = %v", err)) {
				return
			}

			if err != nil {
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			if tt.wantURL != "" {
				assert.Equal(t, tt.wantURL, got)
			}

			for _, substr := range tt.wantContains {
				assert.Contains(t, got, substr)
			}
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
		setupMock func(*trustmock.Repository, *sessionmock.Repository)
		errAssert assert.ErrorAssertionFunc
	}{
		{
			name: "Invalid JWT",
			jwt:  "invalid.jwt.token",
			setupMock: func(oidcs *trustmock.Repository, sessions *sessionmock.Repository) {
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
			setupMock: func(oidcs *trustmock.Repository, sessions *sessionmock.Repository) {
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
			setupMock: func(oidcs *trustmock.Repository, sessions *sessionmock.Repository) {
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
			setupMock: func(oidcs *trustmock.Repository, sessions *sessionmock.Repository) {
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

			oidcMock := trustmock.NewInMemRepository()
			sessionMock := sessionmock.NewInMemRepository()

			rt := localRoundTripper{
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					b, _ := json.Marshal(oidc.Configuration{
						JwksURI: jwksSrv.URL,
						Issuer:  jwksSrv.URL,
					})
					_, _ = w.Write(b)
				}),
			}

			tt.setupMock(oidcMock, sessionMock)

			cfg := &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
			}

			m, err := session.NewManager(cfg, oidcMock, sessionMock, auditLogger, session.WithTransportCredentials(newTCBuilder(rt)))
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

	m, err := session.NewManager(cfg, trustmock.NewInMemRepository(), sessionmock.NewInMemRepository(), auditLogger)
	assert.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "parsing callback URL")
}

func TestManager_LoadState(t *testing.T) {
	const stateID = "test-state-id"

	state := session.State{
		ID:       stateID,
		TenantID: "tenant-id",
		Expiry:   time.Now().Add(time.Hour),
	}

	t.Run("Success", func(t *testing.T) {
		repo := sessionmock.NewInMemRepository(sessionmock.WithState(state))
		m, err := session.NewManager(&config.SessionManager{CSRFSecretParsed: []byte(testCSRFSecret)}, nil, repo, nil)
		require.NoError(t, err)

		got, err := m.LoadState(t.Context(), stateID)
		require.NoError(t, err)
		assert.Equal(t, stateID, got.ID)
	})

	t.Run("Not found", func(t *testing.T) {
		repo := sessionmock.NewInMemRepository()
		m, err := session.NewManager(&config.SessionManager{CSRFSecretParsed: []byte(testCSRFSecret)}, nil, repo, nil)
		require.NoError(t, err)

		_, err = m.LoadState(t.Context(), "non-existent")
		assert.Error(t, err)
	})
}

func TestManager_FinaliseOIDCLogin_StoreSessionError(t *testing.T) {
	const (
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
		RequestURI:   "http://app.example.com/ui",
		Expiry:       time.Now().Add(time.Hour),
	}

	oidcServer := StartOIDCServer(t, false)
	defer oidcServer.Close()

	auditServer := StartAuditServer(t)
	defer auditServer.Close()

	auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
	require.NoError(t, err)

	jwksURI, err := url.JoinPath(oidcServer.URL, "/.well-known/jwks.json")
	require.NoError(t, err)

	mapping := trust.OIDCMapping{
		IssuerURL:  oidcServer.URL,
		JWKSURI:    jwksURI,
		Properties: map[string]string{},
	}

	cfg := &config.SessionManager{
		SessionDuration:  time.Hour,
		CallbackURL:      callbackURL,
		CSRFSecretParsed: []byte(testCSRFSecret),
	}

	t.Run("Store session error", func(t *testing.T) {
		oidcMock := trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, mapping))
		sessMock := sessionmock.NewInMemRepository(
			sessionmock.WithState(validState),
			sessionmock.WithStoreSessionError(errors.New("store failed")),
		)

		m, err := session.NewManager(cfg, oidcMock, sessMock, auditLogger, session.WithAllowHttpScheme(true))
		require.NoError(t, err)

		_, err = m.FinaliseOIDCLogin(context.Background(), stateID, code, fingerprint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storing session")
	})

	t.Run("Bump active error", func(t *testing.T) {
		oidcMock := trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, mapping))
		sessMock := sessionmock.NewInMemRepository(
			sessionmock.WithState(validState),
			sessionmock.WithBumpActiveError(errors.New("bump failed")),
		)

		m, err := session.NewManager(cfg, oidcMock, sessMock, auditLogger, session.WithAllowHttpScheme(true))
		require.NoError(t, err)

		_, err = m.FinaliseOIDCLogin(context.Background(), stateID, code, fingerprint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bumping session active status")
	})

	t.Run("Delete state error", func(t *testing.T) {
		oidcMock := trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, mapping))
		sessMock := sessionmock.NewInMemRepository(
			sessionmock.WithState(validState),
			sessionmock.WithDeleteStateError(errors.New("delete state failed")),
		)

		m, err := session.NewManager(cfg, oidcMock, sessMock, auditLogger, session.WithAllowHttpScheme(true))
		require.NoError(t, err)

		_, err = m.FinaliseOIDCLogin(context.Background(), stateID, code, fingerprint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deleting state")
	})

	t.Run("Missing auth context parameter", func(t *testing.T) {
		oidcMock := trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, mapping))
		sessMock := sessionmock.NewInMemRepository(sessionmock.WithState(validState))

		cfgWithAuthCtx := &config.SessionManager{
			SessionDuration:           time.Hour,
			CallbackURL:               callbackURL,
			CSRFSecretParsed:          []byte(testCSRFSecret),
			AdditionalAuthContextKeys: []string{"nonExistentKey"},
		}

		m, err := session.NewManager(cfgWithAuthCtx, oidcMock, sessMock, auditLogger, session.WithAllowHttpScheme(true))
		require.NoError(t, err)

		result, err := m.FinaliseOIDCLogin(context.Background(), stateID, code, fingerprint)
		assert.NoError(t, err)
		assert.NotEmpty(t, result.SessionID)
	})
}

func TestManager_BCLogout_TrustAndVerifyErrors(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	const keyID = "kid1"
	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.RS256,
		Key: jose.JSONWebKey{
			Key:       key,
			KeyID:     keyID,
			Algorithm: string(jose.RS256),
		},
	}, (&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)

	newJwt := func(claims any) string {
		token, err := jwt.Signed(signer).Claims(claims).Serialize()
		require.NoError(t, err)
		return token
	}

	publicJwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &key.PublicKey,
		KeyID:     keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}}

	var jwksSrv *httptest.Server
	jwksSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(oidc.Configuration{
				Issuer:  jwksSrv.URL,
				JwksURI: jwksSrv.URL + "/jwks",
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(publicJwks)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jwksSrv.Close()

	now := time.Now()
	validLogoutClaims := map[string]any{
		"iss":    jwksSrv.URL,
		"sub":    "user-1",
		"aud":    []string{""},
		"exp":    now.Add(time.Hour).Unix(),
		"iat":    now.Unix(),
		"sid":    "sid-1",
		"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}},
	}

	auditServer := StartAuditServer(t)
	defer auditServer.Close()

	auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
	require.NoError(t, err)

	t.Run("Trust mapping get error", func(t *testing.T) {
		oidcMock := trustmock.NewInMemRepository(trustmock.WithGetError(errors.New("trust error")))
		sessMock := sessionmock.NewInMemRepository()
		_ = sessMock.StoreSession(context.Background(), session.Session{
			ID: "s1", ProviderID: "sid-1", TenantID: "tid-1",
		})

		m, err := session.NewManager(&config.SessionManager{}, oidcMock, sessMock, auditLogger, session.WithAllowHttpScheme(true))
		require.NoError(t, err)

		err = m.BCLogout(t.Context(), newJwt(validLogoutClaims))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "getting trust mapping")
	})

	t.Run("Verify logout token error - wrong issuer", func(t *testing.T) {
		oidcMock := trustmock.NewInMemRepository()
		_ = oidcMock.Create(context.Background(), "tid-1", trust.OIDCMapping{IssuerURL: jwksSrv.URL})
		sessMock := sessionmock.NewInMemRepository()
		_ = sessMock.StoreSession(context.Background(), session.Session{
			ID: "s1", ProviderID: "sid-1", TenantID: "tid-1",
		})

		wrongIssClaims := map[string]any{
			"iss":    "https://wrong-issuer.example.com",
			"sub":    "user-1",
			"aud":    []string{""},
			"exp":    now.Add(time.Hour).Unix(),
			"iat":    now.Unix(),
			"sid":    "sid-1",
			"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}},
		}

		m, err := session.NewManager(&config.SessionManager{}, oidcMock, sessMock, auditLogger, session.WithAllowHttpScheme(true))
		require.NoError(t, err)

		err = m.BCLogout(t.Context(), newJwt(wrongIssClaims))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "verifying logout token")
	})

	t.Run("Delete session error after verify", func(t *testing.T) {
		oidcMock := trustmock.NewInMemRepository()
		_ = oidcMock.Create(context.Background(), "tid-1", trust.OIDCMapping{IssuerURL: jwksSrv.URL})
		sessMock := sessionmock.NewInMemRepository(
			sessionmock.WithDeleteSessionError(errors.New("delete failed")),
		)
		_ = sessMock.StoreSession(context.Background(), session.Session{
			ID: "s1", ProviderID: "sid-1", TenantID: "tid-1",
		})

		m, err := session.NewManager(&config.SessionManager{}, oidcMock, sessMock, auditLogger, session.WithAllowHttpScheme(true))
		require.NoError(t, err)

		err = m.BCLogout(t.Context(), newJwt(validLogoutClaims))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deleting session")
	})
}

func TestManager_MakeAuthURI_MissingAuthParameter(t *testing.T) {
	oidcServer := StartOIDCServer(t, false)
	defer oidcServer.Close()

	oidcMock := trustmock.NewInMemRepository(trustmock.WithTrust("tid", trust.OIDCMapping{
		IssuerURL:  oidcServer.URL,
		Properties: map[string]string{},
	}))

	cfg := &config.SessionManager{
		SessionDuration:                    time.Hour,
		CallbackURL:                        "http://localhost/callback",
		CSRFSecretParsed:                   []byte(testCSRFSecret),
		AdditionalQueryParametersAuthorize: []string{"missingParam"},
	}

	m, err := session.NewManager(cfg, oidcMock, sessionmock.NewInMemRepository(), nil, session.WithAllowHttpScheme(true))
	require.NoError(t, err)

	got, _, err := m.MakeAuthURI(t.Context(), "tid", "fp", "http://app/ui")
	assert.NoError(t, err)
	assert.NotEmpty(t, got)

	u, err := url.Parse(got)
	require.NoError(t, err)
	assert.Empty(t, u.Query().Get("missingParam"), "missing param should not appear in URL")
}

func TestManager_GetClientID_FromMapping(t *testing.T) {
	oidcServer := StartOIDCServer(t, false)
	defer oidcServer.Close()

	mappingClientID := "mapping-specific-client-id"
	oidcMock := trustmock.NewInMemRepository(trustmock.WithTrust("tid", trust.OIDCMapping{
		IssuerURL:  oidcServer.URL,
		ClientID:   mappingClientID,
		Properties: map[string]string{},
	}))

	cfg := &config.SessionManager{
		SessionDuration:  time.Hour,
		CallbackURL:      "http://localhost/callback",
		CSRFSecretParsed: []byte(testCSRFSecret),
		ClientAuth:       config.ClientAuth{ClientID: "global-client-id"},
	}

	m, err := session.NewManager(cfg, oidcMock, sessionmock.NewInMemRepository(), nil, session.WithAllowHttpScheme(true))
	require.NoError(t, err)

	got, _, err := m.MakeAuthURI(t.Context(), "tid", "fp", "http://app/ui")
	require.NoError(t, err)

	u, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, mappingClientID, u.Query().Get("client_id"))
}

func TestAppIDTokenClaims_UnmarshalJSON_Error(t *testing.T) {
	var claims session.AppIDTokenClaims
	err := claims.UnmarshalJSON([]byte(`{invalid json`))
	require.Error(t, err)
}

func TestAppIDTokenClaims_UnmarshalJSON_NoCustomClaims(t *testing.T) {
	payload := `{"iss":"https://example.com","sub":"user1","aud":["client"],"exp":9999999999,"iat":1700000000}`
	var claims session.AppIDTokenClaims
	err := claims.UnmarshalJSON([]byte(payload))
	require.NoError(t, err)
	assert.Empty(t, claims.UserUUID)
	assert.Empty(t, claims.Groups)
}

func TestManager_FinaliseOIDCLogin_NilAuditLogger(t *testing.T) {
	const (
		stateID     = "test-state-id"
		tenantID    = "tenant-id"
		fingerprint = "test-fingerprint"
	)

	expiredState := session.State{
		ID:          stateID,
		TenantID:    tenantID,
		Fingerprint: fingerprint,
		Expiry:      time.Now().Add(-time.Hour),
	}

	sessMock := sessionmock.NewInMemRepository(sessionmock.WithState(expiredState))
	oidcMock := trustmock.NewInMemRepository()

	cfg := &config.SessionManager{
		CSRFSecretParsed: []byte(testCSRFSecret),
	}

	m, err := session.NewManager(cfg, oidcMock, sessMock, nil)
	require.NoError(t, err)

	_, err = m.FinaliseOIDCLogin(context.Background(), stateID, "code", fingerprint)
	assert.Error(t, err)
}

func TestManager_FinaliseOIDCLogin_AuditSendSuccess(t *testing.T) {
	const (
		stateID     = "test-state-id"
		tenantID    = "tenant-id"
		fingerprint = "test-fingerprint"
	)

	validState := session.State{
		ID:           stateID,
		TenantID:     tenantID,
		Fingerprint:  "wrong-fingerprint",
		PKCEVerifier: "test-verifier",
		Expiry:       time.Now().Add(time.Hour),
	}

	auditServer := StartAuditServer(t)
	defer auditServer.Close()

	auditLogger, err := otlpaudit.NewLogger(&commoncfg.Audit{Endpoint: auditServer.URL})
	require.NoError(t, err)

	sessMock := sessionmock.NewInMemRepository(sessionmock.WithState(validState))
	oidcMock := trustmock.NewInMemRepository()

	cfg := &config.SessionManager{
		CSRFSecretParsed: []byte(testCSRFSecret),
	}

	m, err := session.NewManager(cfg, oidcMock, sessMock, auditLogger)
	require.NoError(t, err)

	_, err = m.FinaliseOIDCLogin(context.Background(), stateID, "code", fingerprint)
	assert.Error(t, err)
}

type localRoundTripper struct {
	handler http.Handler
}

func (l localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	l.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

type transportCredentials struct {
	rt localRoundTripper
}

func (tc transportCredentials) Transport() http.RoundTripper {
	return tc.rt
}

func newTCBuilder(rt localRoundTripper) credentials.Builder {
	return func(clientID string) credentials.TransportCredentials {
		return transportCredentials{
			rt: rt,
		}
	}
}
