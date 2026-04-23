package business

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
)

func TestLoadHTTPClient_MTLS(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type: "mtls",
				MTLS: &commoncfg.MTLS{
					Cert:    commoncfg.SourceRef{File: commoncfg.CredentialFile{Path: "/nonexistent/cert.pem"}},
					CertKey: commoncfg.SourceRef{File: commoncfg.CredentialFile{Path: "/nonexistent/key.pem"}},
				},
			},
		},
	}

	// This will fail without actual cert files, but tests the logic path
	_, err := newCredsBuilder(cfg)
	// We expect an error since we don't have real cert files
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load mTLS config")
}

func TestLoadHTTPClient_ClientSecret(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type:         "client_secret",
				ClientSecret: commoncfg.SourceRef{Source: "embedded", Value: "test-secret"},
			},
		},
	}

	builder, err := newCredsBuilder(cfg)
	require.NoError(t, err)
	require.NotNil(t, builder)

	// Verify it's using our custom transport
	creds := builder("test-client")
	clientSecretCreds, ok := creds.(*credentials.ClientSecretPost)
	require.True(t, ok)

	assert.Equal(t, "test-client", clientSecretCreds.ClientID)
	assert.Equal(t, "test-secret", clientSecretCreds.ClientSecret)
}

func TestLoadHTTPClient_Insecure(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type: "insecure",
			},
		},
	}

	builder, err := newCredsBuilder(cfg)
	require.NoError(t, err)
	assert.IsType(t, &credentials.Insecure{}, builder(""))
}

func TestLoadHTTPClient_UnknownType(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type: "unknown",
			},
		},
	}

	_, err := newCredsBuilder(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown Client Auth type")
}

func TestClientAuthRoundTripper_RoundTrip(t *testing.T) {
	tests := []struct {
		name              string
		clientID          string
		clientSecret      string
		requestURL        string
		expectedClientID  string
		expectedHasSecret bool
		expectedSecretVal string
		body              io.Reader
	}{
		{
			name:              "With client secret",
			clientID:          "my-client",
			clientSecret:      "my-secret",
			requestURL:        "https://example.com/token",
			expectedClientID:  "my-client",
			expectedHasSecret: true,
			expectedSecretVal: "my-secret",
		},
		{
			name:              "Without client secret",
			clientID:          "my-client",
			clientSecret:      "",
			requestURL:        "https://example.com/token",
			expectedClientID:  "my-client",
			expectedHasSecret: false,
		},
		{
			name:              "With existing query params",
			clientID:          "my-client",
			clientSecret:      "my-secret",
			requestURL:        "https://example.com/token",
			expectedClientID:  "my-client",
			expectedHasSecret: true,
			expectedSecretVal: "my-secret",
			body:              strings.NewReader("foo=bar"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			// Create a test server that captures the request
			var capturedReq *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				capturedReq = r
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create the round tripper
			creds := credentials.NewClientSecretPost(tt.clientID, tt.clientSecret)

			// Parse the test URL
			reqURL, err := url.Parse(tt.requestURL)
			require.NoError(t, err)

			// Update URL to point to test server
			reqURL.Scheme = "http"
			reqURL.Host = server.Listener.Addr().String()

			// Create and execute request
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, reqURL.String(), tt.body)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			require.NoError(t, err)

			resp, err := creds.Transport().RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			defer resp.Body.Close()

			// Verify the captured request has correct query params
			require.NotNil(t, capturedReq)

			assert.Equal(t, tt.expectedClientID, capturedReq.FormValue("client_id"))

			if tt.expectedHasSecret {
				assert.Equal(t, tt.expectedSecretVal, capturedReq.FormValue("client_secret"))
			} else {
				assert.Empty(t, capturedReq.FormValue("client_secret"))
			}

			// Verify original query params are preserved
			if tt.body != nil {
				b, _ := io.ReadAll(tt.body)
				q, _ := url.ParseQuery(string(b))

				for k, v := range q {
					assert.Equal(t, v, capturedReq.FormValue(k))
				}
			}
		})
	}
}

func TestClientAuthRoundTripper_RoundTrip_PreservesExistingParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "my-client", r.FormValue("client_id"))
		assert.Equal(t, "my-secret", r.FormValue("client_secret"))
		assert.Equal(t, "bar", r.FormValue("foo"))
		assert.Equal(t, "baz", r.FormValue("param2"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	creds := credentials.NewClientSecretPost("my-client", "my-secret")

	reqURL := server.URL
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, reqURL, strings.NewReader("foo=bar&param2=baz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	require.NoError(t, err)

	resp, err := creds.Transport().RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestValkeyClientFromConfig_InvalidHostRef(t *testing.T) {
	cfg := &config.Config{
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	_, err := valkeyClientFromConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load valkey host")
}

func TestValkeyClientFromConfig_InvalidUserRef(t *testing.T) {
	cfg := &config.Config{
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	_, err := valkeyClientFromConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load valkey username")
}

func TestValkeyClientFromConfig_InvalidPasswordRef(t *testing.T) {
	cfg := &config.Config{
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
		},
	}

	_, err := valkeyClientFromConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load valkey password")
}

func TestValkeyClientFromConfig_WithMTLS(t *testing.T) {
	cfg := &config.Config{
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
			SecretRef: commoncfg.SecretRef{
				Type: commoncfg.MTLSSecretType,
				MTLS: commoncfg.MTLS{
					Cert:    commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/cert.pem"}},
					CertKey: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/key.pem"}},
				},
			},
		},
	}

	_, err := valkeyClientFromConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load valkey mTLS config from secret ref")
}

func TestTrustRepoFromConfig_InvalidDatabaseConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	_, err := trustRepoFromConfig(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to make dsn from config")
}

func TestInitSessionManager_InvalidOIDCConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	_, closeFn, err := initSessionManager(t.Context(), cfg)
	assert.Error(t, err)
	assert.Nil(t, closeFn)
	assert.Contains(t, err.Error(), "failed to create trust repository")
}

func TestInitSessionManager_InvalidValkeyConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost"},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	_, closeFn, err := initSessionManager(t.Context(), cfg)
	assert.Error(t, err)
	assert.Nil(t, closeFn)
	// Will fail on either DB connection or valkey config
	// Error details depend on which step fails
}

func TestInitSessionManager_InvalidHTTPClientConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost"},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type: "invalid-type",
			},
		},
	}

	_, closeFn, err := initSessionManager(t.Context(), cfg)
	assert.Error(t, err)
	assert.Nil(t, closeFn)
	// Should fail on one of the earlier steps (DB or valkey) or on HTTP client
	// Error details depend on which step fails
}

func TestPublicMain_InvalidCSRFSecret(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			CSRFSecret: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
		},
	}

	err := publicMain(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading csrf token from source ref")
}

func TestPublicMain_ShortCSRFSecret(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			CSRFSecret: commoncfg.SourceRef{Source: "embedded", Value: "short"},
		},
	}

	err := publicMain(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CSRF secret must be at least 32 bytes")
}

func TestInternalMain_InvalidOIDCConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	err := internalMain(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create trust service")
}

func TestInternalMain_InvalidValkeyConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost"},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	err := internalMain(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create valkey client")
	// Could fail on OIDC (DB connection) or valkey
	// Error details depend on which step fails
}

func TestMain_InvalidCSRFSecret(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			CSRFSecret: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
		},
	}

	err := Main(t.Context(), cfg)
	assert.Error(t, err)
}

func TestMain_PublicServerInvalidCSRF(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			CSRFSecret: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
		},
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost"},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	err := Main(t.Context(), cfg)
	assert.Error(t, err)
}

func TestMain_InternalServerInvalidDatabase(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			CSRFSecret: commoncfg.SourceRef{Source: "embedded", Value: "this-is-a-very-long-secret-that-is-at-least-32-bytes-long"},
			ClientAuth: config.ClientAuth{
				Type: "insecure",
			},
		},
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	err := Main(t.Context(), cfg)
	assert.Error(t, err)
}
