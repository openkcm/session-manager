package grpc_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/openid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/session/v1"

	"github.com/openkcm/session-manager/internal/grpc"
	"github.com/openkcm/session-manager/internal/session"
	sessionmock "github.com/openkcm/session-manager/internal/session/mock"
	"github.com/openkcm/session-manager/internal/trust"
	"github.com/openkcm/session-manager/internal/trust/trustmock"
)

func TestNewSessionServer(t *testing.T) {
	t.Run("creates server successfully", func(t *testing.T) {
		sessionRepo := sessionmock.NewInMemRepository()
		providerRepo := trustmock.NewInMemRepository()
		httpClient := &http.Client{}
		idleSessionTimeout := 90 * time.Minute

		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, idleSessionTimeout)

		assert.NotNil(t, server)
	})

	t.Run("creates server with options", func(t *testing.T) {
		sessionRepo := sessionmock.NewInMemRepository()
		providerRepo := trustmock.NewInMemRepository()
		httpClient := &http.Client{}
		idleSessionTimeout := 90 * time.Minute

		server := grpc.NewSessionServer(
			sessionRepo,
			providerRepo,
			httpClient,
			idleSessionTimeout,
			grpc.WithQueryParametersIntrospect([]string{"param1", "param2"}),
		)

		assert.NotNil(t, server)
	})

	t.Run("handles nil option gracefully", func(t *testing.T) {
		sessionRepo := sessionmock.NewInMemRepository()
		providerRepo := trustmock.NewInMemRepository()
		httpClient := &http.Client{}
		idleSessionTimeout := 90 * time.Minute

		server := grpc.NewSessionServer(
			sessionRepo,
			providerRepo,
			httpClient,
			idleSessionTimeout,
			nil,
		)

		assert.NotNil(t, server)
	})
}

func TestGetSession(t *testing.T) {
	ctx := t.Context()

	t.Run("success - valid session with introspection", func(t *testing.T) {
		// Setup test server for OIDC endpoints
		var testServer *httptest.Server
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/.well-known/openid-configuration":
				_ = json.NewEncoder(w).Encode(openid.Configuration{
					Issuer:                testServer.URL,
					IntrospectionEndpoint: testServer.URL + "/introspect",
				})
			case "/introspect":
				_ = json.NewEncoder(w).Encode(openid.IntrospectResponse{
					Active: true,
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer testServer.Close()

		sess := session.Session{
			ID:          "session-123",
			TenantID:    "tenant-123",
			Fingerprint: "fingerprint-123",
			Issuer:      testServer.URL,
			AccessToken: "access-token-123",
			Claims: session.Claims{
				Subject:    "user-123",
				GivenName:  "John",
				FamilyName: "Doe",
				Email:      "john.doe@example.com",
				Groups:     []string{"group1", "group2"},
			},
			AuthContext: map[string]string{"key": "value"},
		}

		provider := trust.Provider{
			IssuerURL: testServer.URL,
			Blocked:   false,
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		// Mark session as active
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust(sess.TenantID, provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-123",
			TenantId:    "tenant-123",
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetValid())
		assert.Equal(t, testServer.URL, resp.GetIssuer())
		assert.Equal(t, "user-123", resp.GetSubject())
		assert.Equal(t, "John", resp.GetGivenName())
		assert.Equal(t, "Doe", resp.GetFamilyName())
		assert.Equal(t, "john.doe@example.com", resp.GetEmail())
		assert.Equal(t, []string{"group1", "group2"}, resp.GetGroups())
		assert.Equal(t, map[string]string{"key": "value"}, resp.GetAuthContext())
	})

	t.Run("success - introspection returns groups overriding session groups", func(t *testing.T) {
		// Setup test server for OIDC endpoints that returns groups in introspection
		var testServer *httptest.Server
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/.well-known/openid-configuration":
				_ = json.NewEncoder(w).Encode(openid.Configuration{
					Issuer:                testServer.URL,
					IntrospectionEndpoint: testServer.URL + "/introspect",
				})
			case "/introspect":
				_ = json.NewEncoder(w).Encode(openid.IntrospectResponse{
					Active: true,
					Groups: []string{"introspect-group1", "introspect-group2"},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer testServer.Close()

		sess := session.Session{
			ID:          "session-groups",
			TenantID:    "tenant-groups",
			Fingerprint: "fingerprint-groups",
			Issuer:      testServer.URL,
			AccessToken: "access-token-groups",
			Claims: session.Claims{
				Subject: "user-groups",
				Groups:  []string{"session-group1", "session-group2"},
			},
		}

		provider := trust.Provider{
			IssuerURL: testServer.URL,
			Blocked:   false,
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust(sess.TenantID, provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-groups",
			TenantId:    "tenant-groups",
			Fingerprint: "fingerprint-groups",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetValid())
		// Groups should be overridden by introspection result
		assert.Equal(t, []string{"introspect-group1", "introspect-group2"}, resp.GetGroups())
	})

	t.Run("success - valid session without introspection endpoint", func(t *testing.T) {
		// Setup test server without introspection endpoint
		var testServer *httptest.Server
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/.well-known/openid-configuration" {
				_ = json.NewEncoder(w).Encode(openid.Configuration{
					Issuer: testServer.URL,
					// No IntrospectionEndpoint
				})
			}
		}))
		defer testServer.Close()

		sess := session.Session{
			ID:          "session-456",
			TenantID:    "tenant-456",
			Fingerprint: "fingerprint-456",
			Issuer:      testServer.URL,
			Claims: session.Claims{
				Subject: "user-456",
			},
		}

		provider := trust.Provider{
			IssuerURL: testServer.URL,
			Blocked:   false,
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust(sess.TenantID, provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-456",
			TenantId:    "tenant-456",
			Fingerprint: "fingerprint-456",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.GetValid())
	})

	t.Run("invalid - IsActive returns error", func(t *testing.T) {
		isActiveErr := errors.New("database error")
		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithIsActiveError(isActiveErr),
		)
		providerRepo := trustmock.NewInMemRepository()

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-123",
			TenantId:    "tenant-123",
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("invalid - session not active", func(t *testing.T) {
		sess := session.Session{
			ID:          "session-789",
			TenantID:    "tenant-789",
			Fingerprint: "fingerprint-789",
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		// Don't bump active - session is not active

		providerRepo := trustmock.NewInMemRepository()

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-789",
			TenantId:    "tenant-789",
			Fingerprint: "fingerprint-789",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("invalid - LoadSession returns error", func(t *testing.T) {
		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithLoadSessionError(errors.New("load error")),
		)
		// Create a session and mark as active but LoadSession will error
		sess := session.Session{ID: "session-fail"}
		err := sessionRepo.StoreSession(ctx, sess)
		assert.NoError(t, err)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		providerRepo := trustmock.NewInMemRepository()

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-fail",
			TenantId:    "tenant-123",
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("invalid - OIDC provider not found", func(t *testing.T) {
		sess := session.Session{
			ID:          "session-no-provider",
			TenantID:    "tenant-no-provider",
			Fingerprint: "fingerprint-123",
			Issuer:      "https://issuer.example.com",
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		// No provider added to repo
		providerRepo := trustmock.NewInMemRepository()

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-no-provider",
			TenantId:    "tenant-no-provider",
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("invalid - OIDC provider is blocked", func(t *testing.T) {
		sess := session.Session{
			ID:          "session-blocked",
			TenantID:    "tenant-blocked",
			Fingerprint: "fingerprint-123",
			Issuer:      "https://issuer.example.com",
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		provider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
			Blocked:   true, // Provider is blocked
		}
		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust(sess.TenantID, provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-blocked",
			TenantId:    "tenant-blocked",
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("invalid - fingerprint mismatch", func(t *testing.T) {
		sess := session.Session{
			ID:          "session-fingerprint",
			TenantID:    "tenant-fingerprint",
			Fingerprint: "correct-fingerprint",
			Issuer:      "https://issuer.example.com",
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		provider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
			Blocked:   false,
		}
		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust(sess.TenantID, provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-fingerprint",
			TenantId:    "tenant-fingerprint",
			Fingerprint: "wrong-fingerprint", // Mismatch
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("invalid - tenant ID mismatch", func(t *testing.T) {
		sess := session.Session{
			ID:          "session-tenant",
			TenantID:    "correct-tenant",
			Fingerprint: "fingerprint-123",
			Issuer:      "https://issuer.example.com",
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		provider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
			Blocked:   false,
		}
		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust("wrong-tenant", provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-tenant",
			TenantId:    "wrong-tenant", // Mismatch
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("error - GetOpenIDConfig fails", func(t *testing.T) {
		sess := session.Session{
			ID:          "session-config-fail",
			TenantID:    "tenant-config-fail",
			Fingerprint: "fingerprint-123",
			Issuer:      "https://invalid-issuer-no-server.example.com",
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		provider := trust.Provider{
			IssuerURL: "https://invalid-issuer-no-server.example.com",
			Blocked:   false,
		}
		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust(sess.TenantID, provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-config-fail",
			TenantId:    "tenant-config-fail",
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.Error(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("error - introspection fails", func(t *testing.T) {
		// Setup test server that fails introspection
		var testServer *httptest.Server
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/.well-known/openid-configuration":
				_ = json.NewEncoder(w).Encode(openid.Configuration{
					Issuer:                testServer.URL,
					IntrospectionEndpoint: testServer.URL + "/introspect",
				})
			case "/introspect":
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer testServer.Close()

		sess := session.Session{
			ID:          "session-introspect-fail",
			TenantID:    "tenant-introspect-fail",
			Fingerprint: "fingerprint-123",
			Issuer:      testServer.URL,
			AccessToken: "access-token-123",
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		provider := trust.Provider{
			IssuerURL: testServer.URL,
			Blocked:   false,
		}
		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust(sess.TenantID, provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-introspect-fail",
			TenantId:    "tenant-introspect-fail",
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.Error(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("invalid - token not active", func(t *testing.T) {
		// Setup test server that returns inactive token
		var testServer *httptest.Server
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/.well-known/openid-configuration":
				_ = json.NewEncoder(w).Encode(openid.Configuration{
					Issuer:                testServer.URL,
					IntrospectionEndpoint: testServer.URL + "/introspect",
				})
			case "/introspect":
				_ = json.NewEncoder(w).Encode(openid.IntrospectResponse{
					Active: false, // Token is not active
				})
			}
		}))
		defer testServer.Close()

		sess := session.Session{
			ID:          "session-inactive-token",
			TenantID:    "tenant-inactive-token",
			Fingerprint: "fingerprint-123",
			Issuer:      testServer.URL,
			AccessToken: "expired-token",
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		provider := trust.Provider{
			IssuerURL: testServer.URL,
			Blocked:   false,
		}
		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust(sess.TenantID, provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-inactive-token",
			TenantId:    "tenant-inactive-token",
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})

	t.Run("invalid - BumpActive fails", func(t *testing.T) {
		var testServer *httptest.Server
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/.well-known/openid-configuration" {
				_ = json.NewEncoder(w).Encode(openid.Configuration{
					Issuer: testServer.URL,
				})
			}
		}))
		defer testServer.Close()

		sess := session.Session{
			ID:          "session-bump-fail",
			TenantID:    "tenant-bump-fail",
			Fingerprint: "fingerprint-123",
			Issuer:      testServer.URL,
		}

		sessionRepo := sessionmock.NewInMemRepository(
			sessionmock.WithSession(sess),
			sessionmock.WithBumpActiveError(errors.New("bump active error")),
		)
		_ = sessionRepo.BumpActive(ctx, sess.ID, 1*time.Hour)

		provider := trust.Provider{
			IssuerURL: testServer.URL,
			Blocked:   false,
		}
		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust(sess.TenantID, provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetSessionRequest{
			SessionId:   "session-bump-fail",
			TenantId:    "tenant-bump-fail",
			Fingerprint: "fingerprint-123",
		}

		resp, err := server.GetSession(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.GetValid())
	})
}

func TestWithQueryParametersIntrospect(t *testing.T) {
	t.Run("sets query parameters correctly", func(t *testing.T) {
		params := []string{"param1", "param2", "param3"}
		opt := grpc.WithQueryParametersIntrospect(params)

		assert.NotNil(t, opt)

		// Test that the option actually sets the parameters
		sessionRepo := sessionmock.NewInMemRepository()
		providerRepo := trustmock.NewInMemRepository()
		httpClient := &http.Client{}

		server := grpc.NewSessionServer(
			sessionRepo,
			providerRepo,
			httpClient,
			90*time.Minute,
			opt,
		)

		assert.NotNil(t, server)
	})
}

func TestGetOIDCProvider(t *testing.T) {
	ctx := t.Context()

	t.Run("success - returns OIDC provider", func(t *testing.T) {
		provider := trust.Provider{
			IssuerURL: "https://issuer.example.com",
			JWKSURI:   "https://issuer.example.com/.well-known/jwks.json",
			Audiences: []string{"audience1", "audience2"},
		}

		sessionRepo := sessionmock.NewInMemRepository()
		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithTrust("tenant-123", provider),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetOIDCProviderRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.GetOIDCProvider(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.GetProvider())
		assert.Equal(t, "https://issuer.example.com", resp.GetProvider().GetIssuerUrl())
		assert.Equal(t, "https://issuer.example.com/.well-known/jwks.json", resp.GetProvider().GetJwksUri())
		assert.Equal(t, []string{"audience1", "audience2"}, resp.GetProvider().GetAudiences())
	})

	t.Run("error - provider not found", func(t *testing.T) {
		sessionRepo := sessionmock.NewInMemRepository()
		providerRepo := trustmock.NewInMemRepository()

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetOIDCProviderRequest{
			TenantId: "non-existent-tenant",
		}

		resp, err := server.GetOIDCProvider(ctx, req)

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "getting odic provider")
	})

	t.Run("error - repository returns error", func(t *testing.T) {
		sessionRepo := sessionmock.NewInMemRepository()
		providerRepo := trustmock.NewInMemRepository(
			trustmock.WithGetError(errors.New("database connection error")),
		)

		httpClient := &http.Client{}
		server := grpc.NewSessionServer(sessionRepo, providerRepo, httpClient, 90*time.Minute)

		req := &sessionv1.GetOIDCProviderRequest{
			TenantId: "tenant-123",
		}

		resp, err := server.GetOIDCProvider(ctx, req)

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "getting odic provider")
	})
}
