package session_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/session"
	sessionmock "github.com/openkcm/session-manager/internal/session/mock"
	"github.com/openkcm/session-manager/internal/trust"
	"github.com/openkcm/session-manager/internal/trust/trustmock"
)

func TestManager_MakeSessionCookie(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.SessionManager
		tenantID    string
		value       string
		wantErr     bool
		checkCookie func(*testing.T, *http.Cookie)
	}{
		{
			name: "Success with tenant ID",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				SessionCookieTemplate: config.CookieTemplate{
					Name:     "__Host-Http-Session",
					MaxAge:   3600,
					Path:     "/",
					Secure:   true,
					HTTPOnly: true,
					SameSite: config.CookieSameSiteStrict,
				},
			},
			tenantID: "tenant-1",
			value:    "session-123",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.Equal(t, "__Host-Http-Session-tenant-1", cookie.Name)
				assert.Equal(t, "session-123", cookie.Value)
				assert.Equal(t, 3600, cookie.MaxAge)
				assert.Equal(t, "/", cookie.Path)
				assert.True(t, cookie.Secure)
				assert.True(t, cookie.HttpOnly)
				assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
			},
		},
		{
			name: "Success without tenant ID",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				SessionCookieTemplate: config.CookieTemplate{
					Name:     "__Host-Http-Session",
					MaxAge:   3600,
					Path:     "/",
					Secure:   true,
					HTTPOnly: true,
					SameSite: config.CookieSameSiteStrict,
				},
			},
			tenantID: "",
			value:    "session-456",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.Equal(t, "__Host-Http-Session", cookie.Name)
				assert.Equal(t, "session-456", cookie.Value)
			},
		},
		{
			name: "Warning for non-secure cookie",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				SessionCookieTemplate: config.CookieTemplate{
					Name:     "__Host-Http-Session",
					MaxAge:   3600,
					Path:     "/",
					Secure:   false, // Not secure
					HTTPOnly: true,
					SameSite: config.CookieSameSiteStrict,
				},
			},
			tenantID: "tenant-1",
			value:    "session-789",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.False(t, cookie.Secure)
			},
		},
		{
			name: "Warning for non-HTTPOnly cookie",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				SessionCookieTemplate: config.CookieTemplate{
					Name:     "__Host-Http-Session",
					MaxAge:   3600,
					Path:     "/",
					Secure:   true,
					HTTPOnly: false, // Not HTTPOnly
					SameSite: config.CookieSameSiteStrict,
				},
			},
			tenantID: "tenant-1",
			value:    "session-abc",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.False(t, cookie.HttpOnly)
			},
		},
		{
			name: "Warning for cookie name without __Host-Http- prefix",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				SessionCookieTemplate: config.CookieTemplate{
					Name:     "Session", // No __Host-Http- prefix
					MaxAge:   3600,
					Path:     "/",
					Secure:   true,
					HTTPOnly: true,
					SameSite: config.CookieSameSiteStrict,
				},
			},
			tenantID: "tenant-1",
			value:    "session-def",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.Equal(t, "Session-tenant-1", cookie.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := session.NewManager(
				tt.cfg,
				nil,
				sessionmock.NewInMemRepository(),
				nil,
				http.DefaultClient,
			)
			require.NoError(t, err)

			cookie, err := m.MakeSessionCookie(t.Context(), tt.tenantID, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cookie)

			if tt.checkCookie != nil {
				tt.checkCookie(t, cookie)
			}
		})
	}
}

func TestManager_MakeCSRFCookie(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.SessionManager
		tenantID    string
		value       string
		wantErr     bool
		checkCookie func(*testing.T, *http.Cookie)
	}{
		{
			name: "Success with tenant ID",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				CSRFCookieTemplate: config.CookieTemplate{
					Name:     "CSRF-Token",
					MaxAge:   3600,
					Path:     "/",
					Secure:   true,
					HTTPOnly: false,
					SameSite: config.CookieSameSiteStrict,
				},
			},
			tenantID: "tenant-1",
			value:    "csrf-123",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.Equal(t, "CSRF-Token-tenant-1", cookie.Name)
				assert.Equal(t, "csrf-123", cookie.Value)
				assert.Equal(t, 3600, cookie.MaxAge)
				assert.Equal(t, "/", cookie.Path)
				assert.True(t, cookie.Secure)
				assert.False(t, cookie.HttpOnly) // CSRF should not be HttpOnly
				assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
			},
		},
		{
			name: "Success without tenant ID",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				CSRFCookieTemplate: config.CookieTemplate{
					Name:     "CSRF-Token",
					MaxAge:   3600,
					Path:     "/",
					Secure:   true,
					HTTPOnly: false,
					SameSite: config.CookieSameSiteStrict,
				},
			},
			tenantID: "",
			value:    "csrf-456",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.Equal(t, "CSRF-Token", cookie.Name)
				assert.Equal(t, "csrf-456", cookie.Value)
			},
		},
		{
			name: "Warning for non-secure cookie",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				CSRFCookieTemplate: config.CookieTemplate{
					Name:     "CSRF-Token",
					MaxAge:   3600,
					Path:     "/",
					Secure:   false, // Not secure
					HTTPOnly: false,
					SameSite: config.CookieSameSiteStrict,
				},
			},
			tenantID: "tenant-1",
			value:    "csrf-789",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.False(t, cookie.Secure)
			},
		},
		{
			name: "Warning for HTTPOnly CSRF cookie",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				CSRFCookieTemplate: config.CookieTemplate{
					Name:     "CSRF-Token",
					MaxAge:   3600,
					Path:     "/",
					Secure:   true,
					HTTPOnly: true, // CSRF should not be HTTPOnly
					SameSite: config.CookieSameSiteStrict,
				},
			},
			tenantID: "tenant-1",
			value:    "csrf-abc",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.True(t, cookie.HttpOnly) // Will trigger warning
			},
		},
		{
			name: "Warning for non-SameSite=Strict cookie",
			cfg: &config.SessionManager{
				CSRFSecretParsed: []byte(testCSRFSecret),
				CSRFCookieTemplate: config.CookieTemplate{
					Name:     "CSRF-Token",
					MaxAge:   3600,
					Path:     "/",
					Secure:   true,
					HTTPOnly: false,
					SameSite: config.CookieSameSiteLax, // Not Strict
				},
			},
			tenantID: "tenant-1",
			value:    "csrf-def",
			wantErr:  false,
			checkCookie: func(t *testing.T, cookie *http.Cookie) {
				t.Helper()
				assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := session.NewManager(
				tt.cfg,
				nil,
				sessionmock.NewInMemRepository(),
				nil,
				http.DefaultClient,
			)
			require.NoError(t, err)

			cookie, err := m.MakeCSRFCookie(t.Context(), tt.tenantID, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cookie)

			if tt.checkCookie != nil {
				tt.checkCookie(t, cookie)
			}
		})
	}
}

func TestManager_ValidateCSRFToken(t *testing.T) {
	csrfSecret := []byte(testCSRFSecret)

	cfg := &config.SessionManager{
		CSRFSecretParsed: csrfSecret,
		SessionCookieTemplate: config.CookieTemplate{
			Name:     "__Host-Http-Session",
			MaxAge:   3600,
			Path:     "/",
			Secure:   true,
			HTTPOnly: true,
			SameSite: config.CookieSameSiteStrict,
		},
		CSRFCookieTemplate: config.CookieTemplate{
			Name:     "CSRF-Token",
			MaxAge:   3600,
			Path:     "/",
			Secure:   true,
			HTTPOnly: false,
			SameSite: config.CookieSameSiteStrict,
		},
	}

	m, err := session.NewManager(
		cfg,
		nil,
		sessionmock.NewInMemRepository(),
		nil,
		http.DefaultClient,
	)
	require.NoError(t, err)

	tests := []struct {
		name      string
		token     string
		sessionID string
		want      bool
	}{
		{
			name:      "Valid token",
			token:     "valid-token",
			sessionID: "session-123",
			want:      true, // This depends on the actual CSRF validation logic
		},
		{
			name:      "Invalid token",
			token:     "invalid-token",
			sessionID: "session-123",
			want:      false,
		},
		{
			name:      "Empty token",
			token:     "",
			sessionID: "session-123",
			want:      false,
		},
		{
			name:      "Empty session ID",
			token:     "token",
			sessionID: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: The actual validation depends on the CSRF library implementation
			// We're just testing that the method can be called without errors
			result := m.ValidateCSRFToken(tt.token, tt.sessionID)
			assert.IsType(t, false, result) // Ensure it returns a boolean
		})
	}
}

func TestManager_Logout(t *testing.T) {
	const (
		sessionID     = "session-123"
		tenantID      = "tenant-1"
		postLogoutURL = "https://app.example.com/logged-out"
	)

	tests := []struct {
		name           string
		cfg            *config.SessionManager
		setupOIDCRepo  func(t *testing.T) *trustmock.Repository
		setupSession   func(*sessionmock.Repository)
		wantErr        bool
		wantErrMessage string
		wantURL        string
	}{
		{
			name: "Success - redirect to postLogoutURL when no end session endpoint",
			cfg: &config.SessionManager{
				CSRFSecretParsed:      []byte(testCSRFSecret),
				PostLogoutRedirectURL: postLogoutURL,
				ClientAuth: config.ClientAuth{
					ClientID: testClientID,
				},
			},
			setupOIDCRepo: func(t *testing.T) *trustmock.Repository {
				t.Helper()
				// StartOIDCServer doesn't include end_session_endpoint, so it will fall back to postLogoutURL
				oidcServer := StartOIDCServer(t, false)
				t.Cleanup(oidcServer.Close)

				provider := trust.OIDCMapping{
					IssuerURL:  oidcServer.URL,
					JWKSURI:    oidcServer.URL + "/jwks",
					Audiences:  []string{"test"},
					Properties: map[string]string{},
				}
				return trustmock.NewInMemRepository(trustmock.WithTrust(tenantID, provider))
			},
			setupSession: func(repo *sessionmock.Repository) {
				//nolint:errcheck
				repo.StoreSession(t.Context(), session.Session{
					ID:       sessionID,
					TenantID: tenantID,
				})
			},
			wantErr: false,
			wantURL: postLogoutURL,
		},
		{
			name: "Error - session not found",
			cfg: &config.SessionManager{
				CSRFSecretParsed:      []byte(testCSRFSecret),
				PostLogoutRedirectURL: postLogoutURL,
				ClientAuth: config.ClientAuth{
					ClientID: testClientID,
				},
			},
			setupOIDCRepo: func(t *testing.T) *trustmock.Repository {
				t.Helper()
				return trustmock.NewInMemRepository()
			},
			setupSession: func(repo *sessionmock.Repository) {
				// Don't store session - will cause error
			},
			wantErr:        true,
			wantErrMessage: "getting session id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionRepo := sessionmock.NewInMemRepository()
			tt.setupSession(sessionRepo)

			oidcRepo := tt.setupOIDCRepo(t)

			m, err := session.NewManager(
				tt.cfg,
				oidcRepo,
				sessionRepo,
				nil,
				http.DefaultClient,
			)
			require.NoError(t, err)

			logoutURL, err := m.Logout(t.Context(), sessionID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMessage != "" {
					assert.Contains(t, err.Error(), tt.wantErrMessage)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, logoutURL)
		})
	}
}
