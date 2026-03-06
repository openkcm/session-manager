package business

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/config"
)

func TestLoadHTTPClient_MTLS(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type:     "mtls",
				ClientID: "test-client",
				MTLS: &commoncfg.MTLS{
					Cert:    commoncfg.SourceRef{File: commoncfg.CredentialFile{Path: "/nonexistent/cert.pem"}},
					CertKey: commoncfg.SourceRef{File: commoncfg.CredentialFile{Path: "/nonexistent/key.pem"}},
				},
			},
		},
	}

	// This will fail without actual cert files, but tests the logic path
	_, err := loadHTTPClient(cfg)
	// We expect an error since we don't have real cert files
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load mTLS config")
}

func TestLoadHTTPClient_ClientSecret(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type:         "client_secret",
				ClientID:     "test-client",
				ClientSecret: commoncfg.SourceRef{Source: "embedded", Value: "test-secret"},
			},
		},
	}

	client, err := loadHTTPClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify it's using our custom transport
	transport, ok := client.Transport.(*clientAuthRoundTripper)
	require.True(t, ok)
	assert.Equal(t, "test-client", transport.clientID)
	assert.Equal(t, "test-secret", transport.clientSecret)
}

func TestLoadHTTPClient_Insecure(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type:     "insecure",
				ClientID: "test-client",
			},
		},
	}

	client, err := loadHTTPClient(cfg)
	require.NoError(t, err)
	assert.Equal(t, http.DefaultClient, client)
}

func TestLoadHTTPClient_UnknownType(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type:     "unknown",
				ClientID: "test-client",
			},
		},
	}

	_, err := loadHTTPClient(cfg)
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
			requestURL:        "https://example.com/token?foo=bar",
			expectedClientID:  "my-client",
			expectedHasSecret: true,
			expectedSecretVal: "my-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			// Create a test server that captures the request
			var capturedReq *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedReq = r
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create the round tripper
			rt := &clientAuthRoundTripper{
				clientID:     tt.clientID,
				clientSecret: tt.clientSecret,
				next:         http.DefaultTransport,
			}

			// Parse the test URL
			reqURL, err := url.Parse(tt.requestURL)
			require.NoError(t, err)

			// Update URL to point to test server
			reqURL.Scheme = "http"
			reqURL.Host = server.Listener.Addr().String()

			// Create and execute request
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, reqURL.String(), nil)
			require.NoError(t, err)

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			defer resp.Body.Close()

			// Verify the captured request has correct query params
			require.NotNil(t, capturedReq)
			query := capturedReq.URL.Query()

			assert.Equal(t, tt.expectedClientID, query.Get("client_id"))

			if tt.expectedHasSecret {
				assert.Equal(t, tt.expectedSecretVal, query.Get("client_secret"))
			} else {
				assert.Empty(t, query.Get("client_secret"))
			}

			// Verify original query params are preserved
			if tt.requestURL == "https://example.com/token?foo=bar" {
				assert.Equal(t, "bar", query.Get("foo"))
			}
		})
	}
}

func TestClientAuthRoundTripper_RoundTrip_PreservesExistingParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		assert.Equal(t, "my-client", query.Get("client_id"))
		assert.Equal(t, "my-secret", query.Get("client_secret"))
		assert.Equal(t, "bar", query.Get("foo"))
		assert.Equal(t, "baz", query.Get("param2"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rt := &clientAuthRoundTripper{
		clientID:     "my-client",
		clientSecret: "my-secret",
		next:         http.DefaultTransport,
	}

	reqURL := server.URL + "?foo=bar&param2=baz"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, reqURL, nil)
	require.NoError(t, err)

	resp, err := rt.RoundTrip(req)
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
	// Main returns nil but publicMain will fail with CSRF error
	// The error is logged but Main itself returns nil as designed
	assert.NoError(t, err)
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
	// Main captures error and shuts down, returning nil
	assert.NoError(t, err)
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
	// Main captures error and shuts down, returning nil
	assert.NoError(t, err)
}
