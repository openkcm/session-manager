package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openkcm/common-sdk/pkg/csrf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/middleware"
	"github.com/openkcm/session-manager/internal/openapi"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/session"
)

// mockSessionManager is a mock implementation of sessionManager interface for testing
type mockSessionManager struct {
	makeAuthURIFunc       func(ctx context.Context, tenantID, fingerprint, requestURI string) (string, error)
	finaliseOIDCLoginFunc func(ctx context.Context, state, code, fingerprint string) (session.OIDCSessionData, error)
	makeSessionCookieFunc func(ctx context.Context, tenantID, sessionID string) (*http.Cookie, error)
	makeCSRFCookieFunc    func(ctx context.Context, tenantID, csrfToken string) (*http.Cookie, error)
	logoutFunc            func(ctx context.Context, sessionID string) (string, error)
	bcLogoutFunc          func(ctx context.Context, logoutToken string) error
}

func (m *mockSessionManager) MakeAuthURI(ctx context.Context, tenantID, fp, requestURI string) (string, error) {
	if m.makeAuthURIFunc != nil {
		return m.makeAuthURIFunc(ctx, tenantID, fp, requestURI)
	}
	return "", errors.New("not implemented")
}

func (m *mockSessionManager) FinaliseOIDCLogin(ctx context.Context, state, code, fp string) (session.OIDCSessionData, error) {
	if m.finaliseOIDCLoginFunc != nil {
		return m.finaliseOIDCLoginFunc(ctx, state, code, fp)
	}
	return session.OIDCSessionData{}, errors.New("not implemented")
}

func (m *mockSessionManager) MakeSessionCookie(ctx context.Context, tenantID, sessionID string) (*http.Cookie, error) {
	if m.makeSessionCookieFunc != nil {
		return m.makeSessionCookieFunc(ctx, tenantID, sessionID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSessionManager) MakeCSRFCookie(ctx context.Context, tenantID, csrfToken string) (*http.Cookie, error) {
	if m.makeCSRFCookieFunc != nil {
		return m.makeCSRFCookieFunc(ctx, tenantID, csrfToken)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSessionManager) Logout(ctx context.Context, sessionID string) (string, error) {
	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, sessionID)
	}
	return "", errors.New("not implemented")
}

func (m *mockSessionManager) BCLogout(ctx context.Context, logoutToken string) error {
	if m.bcLogoutFunc != nil {
		return m.bcLogoutFunc(ctx, logoutToken)
	}
	return errors.New("not implemented")
}

func TestNewOpenAPIServer(t *testing.T) {
	t.Run("creates server with all parameters", func(t *testing.T) {
		csrfSecret := []byte("test-secret")
		sessionCookieName := "session-id"
		csrfCookieName := "csrf-token"

		server := newOpenAPIServer(nil, csrfSecret, sessionCookieName, csrfCookieName)

		assert.NotNil(t, server)
		assert.Equal(t, csrfSecret, server.csrfSecret)
		assert.Equal(t, sessionCookieName, server.sessionIDCookieName)
		assert.Equal(t, csrfCookieName, server.csrfTokenCookieName)
	})
}

func TestOpenAPIServer_Auth_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	server := newOpenAPIServer(nil, nil, "", "")
	req := openapi.AuthRequestObject{}
	resp, err := server.Auth(ctx, req)
	assert.NoError(t, err)

	assert.IsType(t, openapi.AuthdefaultJSONResponse{}, resp)

	r, _ := resp.(openapi.AuthdefaultJSONResponse)
	assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
}

func TestOpenAPIServer_Callback_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	server := newOpenAPIServer(nil, nil, "", "")
	req := openapi.CallbackRequestObject{}
	resp, err := server.Callback(ctx, req)
	assert.NoError(t, err)

	assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)

	r, _ := resp.(openapi.CallbackdefaultJSONResponse)
	assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
}

func TestOpenAPIServer_Callback_NoResponseWriter(t *testing.T) {
	t.Run("returns error when response writer is not in context", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "")
		ctx := t.Context()

		callbackReq := openapi.CallbackRequestObject{
			Params: openapi.CallbackParams{
				State: "state",
				Code:  "code",
			},
		}

		resp, err := server.Callback(ctx, callbackReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)

		r, ok := resp.(openapi.CallbackdefaultJSONResponse)
		require.True(t, ok)
		assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
		assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	})
}

func TestOpenAPIServer_Logout_NoResponseWriter(t *testing.T) {
	t.Run("returns error when response writer is not in context", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "")
		ctx := t.Context()

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				Cookie:     "session-id=123",
				XCSRFToken: "token",
			},
		}

		resp, err := server.Logout(ctx, logoutReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.LogoutdefaultJSONResponse{}, resp)

		r, ok := resp.(openapi.LogoutdefaultJSONResponse)
		require.True(t, ok)
		assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
		assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	})
}

func TestOpenAPIServer_Logout_InvalidCookie(t *testing.T) {
	t.Run("returns error for invalid cookie header", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "")

		rw := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, rw)

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				Cookie:     "invalid cookie format\n\n",
				XCSRFToken: "token",
			},
		}

		resp, err := server.Logout(ctx, logoutReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.LogoutdefaultJSONResponse{}, resp)

		r, ok := resp.(openapi.LogoutdefaultJSONResponse)
		require.True(t, ok)
		assert.Equal(t, string(serviceerr.CodeInvalidRequest), r.Body.Error)
		assert.Equal(t, http.StatusBadRequest, r.StatusCode)
	})
}

func TestOpenAPIServer_Logout_MissingSessionCookie(t *testing.T) {
	t.Run("returns error when session cookie is missing", func(t *testing.T) {
		server := newOpenAPIServer(nil, []byte("secret"), "session-id", "csrf-token")

		rw := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, rw)

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				Cookie:     "csrf-token=some-token",
				XCSRFToken: "some-token",
			},
		}

		resp, err := server.Logout(ctx, logoutReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.LogoutdefaultJSONResponse{}, resp)

		r, ok := resp.(openapi.LogoutdefaultJSONResponse)
		require.True(t, ok)
		assert.Equal(t, string(serviceerr.CodeInvalidRequest), r.Body.Error)
		assert.Equal(t, http.StatusBadRequest, r.StatusCode)
		assert.Contains(t, *r.Body.ErrorDescription, "missing session id")
	})
}

func TestOpenAPIServer_Logout_MissingCSRFCookie(t *testing.T) {
	t.Run("returns error when CSRF cookie is missing", func(t *testing.T) {
		server := newOpenAPIServer(nil, []byte("secret"), "session-id", "csrf-token")

		rw := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, rw)

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				Cookie:     "session-id=session-123",
				XCSRFToken: "some-token",
			},
		}

		resp, err := server.Logout(ctx, logoutReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.LogoutdefaultJSONResponse{}, resp)

		r, ok := resp.(openapi.LogoutdefaultJSONResponse)
		require.True(t, ok)
		assert.Equal(t, string(serviceerr.CodeInvalidRequest), r.Body.Error)
		assert.Equal(t, http.StatusBadRequest, r.StatusCode)
		assert.Contains(t, *r.Body.ErrorDescription, "missing csrf token")
	})
}

func TestOpenAPIServer_Logout_InvalidCSRFToken(t *testing.T) {
	t.Run("returns error when CSRF token is invalid", func(t *testing.T) {
		server := newOpenAPIServer(nil, []byte("test-secret-32-bytes-length!!"), "session-id", "csrf-token")

		rw := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, rw)

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				Cookie:     "session-id=session-123; csrf-token=csrf-123",
				XCSRFToken: "wrong-csrf-token",
			},
		}

		resp, err := server.Logout(ctx, logoutReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.LogoutdefaultJSONResponse{}, resp)

		r, ok := resp.(openapi.LogoutdefaultJSONResponse)
		require.True(t, ok)
		assert.Equal(t, string(serviceerr.CodeInvalidCSRFToken), r.Body.Error)
		assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	})
}

func TestOpenAPIServer_Bclogout(t *testing.T) {
}

func TestOpenAPIServer_ToErrorModel(t *testing.T) {
	server := newOpenAPIServer(nil, nil, "", "")

	t.Run("converts service error to error model", func(t *testing.T) {
		err := serviceerr.ErrUnauthorized
		model, status := server.toErrorModel(err)

		assert.Equal(t, string(serviceerr.CodeUnauthorizedClient), model.Error)
		assert.NotNil(t, model.ErrorDescription)
		assert.Equal(t, http.StatusUnauthorized, status)
	})

	t.Run("converts unknown error to error model", func(t *testing.T) {
		err := errors.New("some random error")
		model, status := server.toErrorModel(err)

		assert.Equal(t, string(serviceerr.CodeUnknown), model.Error)
		assert.NotNil(t, model.ErrorDescription)
		assert.Equal(t, http.StatusInternalServerError, status)
	})

	t.Run("handles pre-defined service errors", func(t *testing.T) {
		testCases := []struct {
			name       string
			err        *serviceerr.Error
			wantCode   serviceerr.Code
			wantStatus int
		}{
			{
				name:       "unauthorized error",
				err:        serviceerr.ErrUnauthorized,
				wantCode:   serviceerr.CodeUnauthorizedClient,
				wantStatus: http.StatusUnauthorized,
			},
			{
				name:       "invalid CSRF token error",
				err:        serviceerr.ErrInvalidCSRFToken,
				wantCode:   serviceerr.CodeInvalidCSRFToken,
				wantStatus: http.StatusInternalServerError, // Default status since not in switch
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				model, status := server.toErrorModel(tc.err)

				assert.Equal(t, string(tc.wantCode), model.Error)
				assert.NotNil(t, model.ErrorDescription)
				assert.Equal(t, tc.wantStatus, status)
			})
		}
	})
}

func TestNewBadRequest(t *testing.T) {
	t.Run("creates bad request error model", func(t *testing.T) {
		description := "invalid input parameter"
		model, status := newBadRequest(description)

		assert.Equal(t, string(serviceerr.CodeInvalidRequest), model.Error)
		assert.NotNil(t, model.ErrorDescription)
		assert.Equal(t, description, *model.ErrorDescription)
		assert.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("creates bad request with different descriptions", func(t *testing.T) {
		testCases := []string{
			"missing required field",
			"invalid format",
			"parameter out of range",
		}

		for _, desc := range testCases {
			model, status := newBadRequest(desc)

			assert.Equal(t, string(serviceerr.CodeInvalidRequest), model.Error)
			assert.Equal(t, desc, *model.ErrorDescription)
			assert.Equal(t, http.StatusBadRequest, status)
		}
	})
}

func TestOpenAPIServer_Bclogout_Success(t *testing.T) {
	t.Run("returns 200 on successful backchannel logout", func(t *testing.T) {
		mock := &mockSessionManager{
			bcLogoutFunc: func(ctx context.Context, logoutToken string) error {
				assert.Equal(t, "valid-logout-token", logoutToken)
				return nil
			},
		}
		server := newOpenAPIServer(mock, nil, "", "")

		bclogoutReq := openapi.BclogoutRequestObject{
			Body: &openapi.BclogoutFormdataRequestBody{
				LogoutToken: "valid-logout-token",
			},
		}

		resp, err := server.Bclogout(context.Background(), bclogoutReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.Bclogout200Response{}, resp)
	})
}

func TestOpenAPIServer_Bclogout_Error(t *testing.T) {
	t.Run("returns 400 when BCLogout fails", func(t *testing.T) {
		mock := &mockSessionManager{
			bcLogoutFunc: func(ctx context.Context, logoutToken string) error {
				return serviceerr.ErrInvalidCSRFToken
			},
		}
		server := newOpenAPIServer(mock, nil, "", "")

		bclogoutReq := openapi.BclogoutRequestObject{
			Body: &openapi.BclogoutFormdataRequestBody{
				LogoutToken: "invalid-token",
			},
		}

		resp, err := server.Bclogout(context.Background(), bclogoutReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.Bclogout400JSONResponse{}, resp)

		r, ok := resp.(openapi.Bclogout400JSONResponse)
		require.True(t, ok)
		assert.Equal(t, string(serviceerr.CodeInvalidCSRFToken), r.Error)
	})
}

func TestOpenAPIServer_Logout_Success(t *testing.T) {
	t.Run("clears cookies and redirects on successful logout", func(t *testing.T) {
		expectedURL := "https://idp.example.com/logout"
		csrfSecret := []byte("secret-key-12345")
		sessionID := "session-123"

		// Generate a valid CSRF token
		validCSRFToken := csrf.NewToken(sessionID, csrfSecret)

		mock := &mockSessionManager{
			logoutFunc: func(ctx context.Context, sid string) (string, error) {
				assert.Equal(t, sessionID, sid)
				return expectedURL, nil
			},
		}
		server := newOpenAPIServer(mock, csrfSecret, "session-id", "csrf-token")

		rw := httptest.NewRecorder()
		ctx := context.WithValue(context.Background(), middleware.ResponseWriterKey, rw)

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				Cookie:     "session-id=" + sessionID + "; csrf-token=" + validCSRFToken,
				XCSRFToken: validCSRFToken,
			},
		}

		resp, err := server.Logout(ctx, logoutReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.Logout302Response{}, resp)

		r, ok := resp.(openapi.Logout302Response)
		require.True(t, ok)
		assert.Equal(t, expectedURL, r.Headers.Location)

		// Verify cookies were cleared
		cookies := rw.Result().Cookies()
		assert.Len(t, cookies, 2)
		for _, cookie := range cookies {
			assert.Equal(t, -1, cookie.MaxAge)
			assert.Empty(t, cookie.Value)
		}
	})
}

// Note: Auth and Callback functions have lower unit test coverage because they depend on
// fingerprint.ExtractFingerprint() from the common-sdk package, which requires proper
// HTTP middleware setup. These functions are more thoroughly tested in integration tests
// where the full HTTP middleware stack is available. The current unit tests cover:
// - Error handling when fingerprint extraction fails
// - Error handling when response writer is not in context
// - Error model conversion
// For full coverage of success paths and session manager interactions, see integration tests.
