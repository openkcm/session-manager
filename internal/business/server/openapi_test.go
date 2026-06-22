package server

import (
	"context"
	"errors"
	"fmt"
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

const (
	allowedBaseURL = "https://app.example.com"
	postLogoutURL  = "https://app.example.com/logged-out"
)

// mockSessionManager is a mock implementation of sessionManager interface for testing
type mockSessionManager struct {
	makeAuthURIFunc         func(ctx context.Context, tenantID, requestURI, errorURI string) (string, string, error)
	finaliseOIDCLoginFunc   func(ctx context.Context, state, code string) (session.OIDCSessionData, error)
	makeSessionCookieFunc   func(ctx context.Context, tenantID, sessionID string) (*http.Cookie, error)
	makeCSRFCookieFunc      func(ctx context.Context, tenantID, csrfToken string) (*http.Cookie, error)
	makeLoginCSRFCookieFunc func(ctx context.Context, csrfToken string) (*http.Cookie, error)
	loadStateFunc           func(ctx context.Context, stateID string) (session.State, error)
	logoutFunc              func(ctx context.Context, sessionID, postLogoutRedirectURL string) (string, error)
	bcLogoutFunc            func(ctx context.Context, logoutToken string) error
}

func (m *mockSessionManager) MakeAuthURI(ctx context.Context, tenantID, requestURI, errorURI string) (string, string, error) {
	if m.makeAuthURIFunc != nil {
		return m.makeAuthURIFunc(ctx, tenantID, requestURI, errorURI)
	}
	return "", "", errors.New("not implemented")
}

func (m *mockSessionManager) FinaliseOIDCLogin(ctx context.Context, state, code string) (session.OIDCSessionData, error) {
	if m.finaliseOIDCLoginFunc != nil {
		return m.finaliseOIDCLoginFunc(ctx, state, code)
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

func (m *mockSessionManager) MakeLoginCSRFCookie(ctx context.Context, csrfToken string) (*http.Cookie, error) {
	if m.makeLoginCSRFCookieFunc != nil {
		return m.makeLoginCSRFCookieFunc(ctx, csrfToken)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSessionManager) LoadState(ctx context.Context, stateID string) (session.State, error) {
	if m.loadStateFunc != nil {
		return m.loadStateFunc(ctx, stateID)
	}
	return session.State{}, errors.New("not implemented")
}

func (m *mockSessionManager) Logout(ctx context.Context, sessionID, postLogoutRedirectURL string) (string, error) {
	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, sessionID, postLogoutRedirectURL)
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

		server := newOpenAPIServer(nil, csrfSecret, sessionCookieName, csrfCookieName, []string{allowedBaseURL})

		assert.NotNil(t, server)
		assert.Equal(t, csrfSecret, server.csrfSecret)
		assert.Equal(t, sessionCookieName, server.sessionIDCookieNamePrefix)
		assert.Equal(t, csrfCookieName, server.csrfTokenCookieNamePrefix)
	})
}

func TestOpenAPIServer_Auth_MakeAuthURI_NilManager(t *testing.T) {
	mock := &mockSessionManager{
		makeAuthURIFunc: func(ctx context.Context, tenantID, requestURI, errorURI string) (string, string, error) {
			return "", "", errors.New("context canceled")
		},
	}
	ctx := t.Context()
	server := newOpenAPIServer(mock, nil, "", "", []string{allowedBaseURL})
	req := openapi.AuthRequestObject{
		Params: openapi.AuthParams{RequestURI: allowedBaseURL + "/page"},
	}
	resp, err := server.Auth(ctx, req)
	assert.NoError(t, err)

	assert.IsType(t, openapi.AuthdefaultJSONResponse{}, resp)

	r, _ := resp.(openapi.AuthdefaultJSONResponse)
	assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
}

func TestOpenAPIServer_Auth_MakeAuthURI_Failed(t *testing.T) {
	mock := &mockSessionManager{
		makeAuthURIFunc: func(ctx context.Context, tenantID, requestURI, errorURI string) (string, string, error) {
			return "", "", errors.New("error")
		},
	}
	ctx := t.Context()
	server := newOpenAPIServer(mock, nil, "", "", []string{allowedBaseURL})
	req := openapi.AuthRequestObject{
		Params: openapi.AuthParams{RequestURI: allowedBaseURL + "/page"},
	}
	resp, err := server.Auth(ctx, req)
	assert.NoError(t, err)

	assert.IsType(t, openapi.AuthdefaultJSONResponse{}, resp)

	r, _ := resp.(openapi.AuthdefaultJSONResponse)
	assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
}

func TestOpenAPIServer_Auth_MakeCSRFCookie_Failed(t *testing.T) {
	mock := &mockSessionManager{
		makeAuthURIFunc: func(ctx context.Context, tenantID, requestURI, errorURI string) (string, string, error) {
			return "https://example.com/redirect", "token", nil
		},
		makeLoginCSRFCookieFunc: func(ctx context.Context, csrfToken string) (*http.Cookie, error) {
			return nil, errors.New("error")
		},
	}
	ctx := t.Context()
	server := newOpenAPIServer(mock, nil, "", "", []string{allowedBaseURL})
	req := openapi.AuthRequestObject{
		Params: openapi.AuthParams{RequestURI: allowedBaseURL + "/page"},
	}
	resp, err := server.Auth(ctx, req)
	assert.NoError(t, err)
	assert.IsType(t, openapi.AuthdefaultJSONResponse{}, resp)

	r, _ := resp.(openapi.AuthdefaultJSONResponse)
	assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
}

func TestOpenAPIServer_Auth_MakeAuthURI_Success(t *testing.T) {
	mock := &mockSessionManager{
		makeAuthURIFunc: func(ctx context.Context, tenantID, requestURI, errorURI string) (string, string, error) {
			return "https://example.com/redirect", "token", nil
		},
		makeLoginCSRFCookieFunc: func(ctx context.Context, csrfToken string) (*http.Cookie, error) {
			return &http.Cookie{Name: "csrf-token", Value: csrfToken}, nil
		},
	}
	ctx := t.Context()
	server := newOpenAPIServer(mock, nil, "", "", []string{"https://example.com"})
	req := openapi.AuthRequestObject{
		Params: openapi.AuthParams{
			RequestURI: "https://example.com/redirect",
		},
	}
	resp, err := server.Auth(ctx, req)
	assert.NoError(t, err)

	assert.IsType(t, openapi.Auth302Response{}, resp)

	r, _ := resp.(openapi.Auth302Response)
	assert.Equal(t, "https://example.com/redirect", r.Headers.Location)
	assert.Equal(t, "csrf-token=token", r.Headers.SetCookie)
}

func TestOpenAPIServer_Callback_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
	req := openapi.CallbackRequestObject{}
	resp, err := server.Callback(ctx, req)
	assert.NoError(t, err)

	assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)

	r, _ := resp.(openapi.CallbackdefaultJSONResponse)
	assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
}

func TestOpenAPIServer_Callback_ExtractFingerprint_Failed(t *testing.T) {
	t.Run("returns an error when response writer is not in the context", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		ctx := t.Context()

		callbackReq := openapi.CallbackRequestObject{
			Params: openapi.CallbackParams{
				State:                             "state",
				Code:                              "code",
				UnderscoreUnderscoreHostLoginCSRF: "session-id=123",
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

func TestOpenAPIServer_Callback_NoResponseWriter(t *testing.T) {
	t.Run("returns error when no response writer", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		ctx := t.Context()

		callbackReq := openapi.CallbackRequestObject{
			Params: openapi.CallbackParams{
				State:                             "state",
				Code:                              "code",
				UnderscoreUnderscoreHostLoginCSRF: "session-id=123",
			},
		}

		resp, err := server.Callback(ctx, callbackReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)

		r, _ := resp.(openapi.CallbackdefaultJSONResponse)
		assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
		assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	})
}

func TestOpenAPIServer_Callback_FinaliseOIDCLogin_Failed(t *testing.T) {
	t.Run("returns an error when FinaliseOIDCLogin failed", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, w)

		csrfSecret := []byte("test-secret")
		state := "state"
		loginCsrfToken := csrf.NewToken(state, csrfSecret)

		mock := &mockSessionManager{
			finaliseOIDCLoginFunc: func(ctx context.Context, state, code string) (session.OIDCSessionData, error) {
				return session.OIDCSessionData{}, serviceerr.ErrAccessDenied
			},
		}

		server := newOpenAPIServer(mock, csrfSecret, "", "", []string{allowedBaseURL})

		callbackReq := openapi.CallbackRequestObject{
			Params: openapi.CallbackParams{
				State:                             "state",
				Code:                              "code",
				UnderscoreUnderscoreHostLoginCSRF: loginCsrfToken,
			},
		}

		resp, err := server.Callback(ctx, callbackReq)

		require.NoError(t, err)
		assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)

		r, _ := resp.(openapi.CallbackdefaultJSONResponse)
		assert.Equal(t, string(serviceerr.CodeUnauthorizedClient), r.Body.Error)
		assert.Equal(t, http.StatusUnauthorized, r.StatusCode)
	})
}

func TestOpenAPIServer_Callback_MakeSessionCookie_Failed(t *testing.T) {
	t.Run("returns an error when MakeSessionCookie failed", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, w)

		csrfSecret := []byte("test-secret")
		state := "state"
		loginCsrfToken := csrf.NewToken(state, csrfSecret)

		mock := &mockSessionManager{
			finaliseOIDCLoginFunc: func(ctx context.Context, state, code string) (session.OIDCSessionData, error) {
				return session.OIDCSessionData{
					SessionID:  "s-id",
					TenantID:   "t-id",
					CSRFToken:  "csrf-token",
					RequestURI: "https://example.com/request",
				}, nil
			},
			makeSessionCookieFunc: func(ctx context.Context, tenantID, sessionID string) (*http.Cookie, error) {
				return nil, errors.New("error")
			},
		}

		server := newOpenAPIServer(mock, csrfSecret, "", "", []string{allowedBaseURL})

		callbackReq := openapi.CallbackRequestObject{
			Params: openapi.CallbackParams{
				State:                             state,
				Code:                              "code",
				UnderscoreUnderscoreHostLoginCSRF: loginCsrfToken,
			},
		}

		resp, err := server.Callback(ctx, callbackReq)
		fmt.Println(resp)

		require.NoError(t, err)
		assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)

		r, _ := resp.(openapi.CallbackdefaultJSONResponse)
		assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
		assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	})
}
func TestOpenAPIServer_Callback_InvalidCsrfToken_Failed(t *testing.T) {
	t.Run("returns an error when login CSRF token is invalid", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, w)

		csrfSecret := []byte("test-secret")

		server := newOpenAPIServer(nil, csrfSecret, "", "", []string{allowedBaseURL})

		callbackReq := openapi.CallbackRequestObject{
			Params: openapi.CallbackParams{
				State:                             "state",
				Code:                              "code",
				UnderscoreUnderscoreHostLoginCSRF: "invalid-csrf-token",
			},
		}

		resp, err := server.Callback(ctx, callbackReq)
		fmt.Println(resp)

		require.NoError(t, err)
		assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)

		r, _ := resp.(openapi.CallbackdefaultJSONResponse)
		assert.Equal(t, string(serviceerr.CodeInvalidLoginCSRFToken), r.Body.Error)
		assert.Equal(t, http.StatusBadRequest, r.StatusCode)
	})
}

func TestOpenAPIServer_Callback_MakeCSRFCookie_Failed(t *testing.T) {
	t.Run("returns an error when MakeCSRFCookie failed", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, w)

		csrfSecret := []byte("test-secret")
		state := "state"
		loginCsrfToken := csrf.NewToken(state, csrfSecret)

		mock := &mockSessionManager{
			finaliseOIDCLoginFunc: func(ctx context.Context, state, code string) (session.OIDCSessionData, error) {
				return session.OIDCSessionData{
					SessionID:  "s-id",
					TenantID:   "t-id",
					CSRFToken:  "csrf-token",
					RequestURI: "https://example.com/request",
				}, nil
			},
			makeSessionCookieFunc: func(ctx context.Context, tenantID, sessionID string) (*http.Cookie, error) {
				return &http.Cookie{Name: "session", Value: "s-id"}, nil
			},
			makeCSRFCookieFunc: func(ctx context.Context, tenantID, csrfToken string) (*http.Cookie, error) {
				return nil, errors.New("error")
			},
		}

		server := newOpenAPIServer(mock, csrfSecret, "", "", []string{allowedBaseURL})

		callbackReq := openapi.CallbackRequestObject{
			Params: openapi.CallbackParams{
				State:                             state,
				Code:                              "code",
				UnderscoreUnderscoreHostLoginCSRF: loginCsrfToken,
			},
		}

		resp, err := server.Callback(ctx, callbackReq)
		fmt.Println(resp)

		require.NoError(t, err)
		assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)

		r, _ := resp.(openapi.CallbackdefaultJSONResponse)
		assert.Equal(t, string(serviceerr.CodeUnknown), r.Body.Error)
		assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	})
}

func TestOpenAPIServer_Callback_Success(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, w)

		csrfSecret := []byte("test-secret")
		state := "state"
		loginCsrfToken := csrf.NewToken(state, csrfSecret)

		mock := &mockSessionManager{
			finaliseOIDCLoginFunc: func(ctx context.Context, state, code string) (session.OIDCSessionData, error) {
				return session.OIDCSessionData{
					SessionID:  "s-id",
					TenantID:   "t-id",
					CSRFToken:  "csrf-token",
					RequestURI: "https://example.com/request",
				}, nil
			},
			makeSessionCookieFunc: func(ctx context.Context, tenantID, sessionID string) (*http.Cookie, error) {
				return &http.Cookie{Name: "session", Value: "s-id"}, nil
			},
			makeCSRFCookieFunc: func(ctx context.Context, tenantID, csrfToken string) (*http.Cookie, error) {
				return &http.Cookie{Name: "csrf", Value: "csrf-token"}, nil
			},
		}

		server := newOpenAPIServer(mock, csrfSecret, "", "", []string{allowedBaseURL})

		callbackReq := openapi.CallbackRequestObject{
			Params: openapi.CallbackParams{
				State:                             state,
				Code:                              "code",
				UnderscoreUnderscoreHostLoginCSRF: loginCsrfToken,
			},
		}

		resp, err := server.Callback(ctx, callbackReq)
		fmt.Println(resp)

		require.NoError(t, err)
		assert.IsType(t, openapi.Callback302Response{}, resp)

		r, _ := resp.(openapi.Callback302Response)
		assert.Equal(t, "https://example.com/request", r.Headers.Location)

		setCookieVals := w.Header().Values("Set-Cookie")
		cookies := make(map[string]string, len(setCookieVals))
		for _, v := range setCookieVals {
			cookie, err := http.ParseSetCookie(v)
			require.NoError(t, err)
			cookies[cookie.Name] = cookie.Value
		}

		assert.Equal(t, "s-id", cookies["session"])
		assert.Equal(t, "csrf-token", cookies["csrf"])
	})
}

func TestOpenAPIServer_Logout_NoResponseWriter(t *testing.T) {
	t.Run("returns an error when response writer is not in context", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		ctx := t.Context()

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				PostLogoutRedirectURI: postLogoutURL,
				Cookie:                "session-id=123",
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
	t.Run("returns an error for invalid cookie header", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})

		rw := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, rw)

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				PostLogoutRedirectURI: postLogoutURL,
				Cookie:                "invalid cookie format\n\n",
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
	t.Run("returns an error when session cookie is missing", func(t *testing.T) {
		server := newOpenAPIServer(nil, []byte("secret"), "session-id", "csrf-token", []string{allowedBaseURL})

		rw := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, rw)

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				PostLogoutRedirectURI: postLogoutURL,
				Cookie:                "csrf-token=some-token",
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

func TestOpenAPIServer_Logout_Failed(t *testing.T) {
	t.Run("returns an error Logout failed", func(t *testing.T) {
		const tokenKey = "test-secret-32-bytes-length!!"
		const tenantID = "tenant-1"
		mock := &mockSessionManager{
			logoutFunc: func(ctx context.Context, sessionID, postLogoutRedirectURL string) (string, error) {
				return "", errors.New("error")
			},
		}

		server := newOpenAPIServer(mock, []byte(tokenKey), "session-id", "csrf-token", []string{allowedBaseURL})

		rw := httptest.NewRecorder()
		ctx := context.WithValue(t.Context(), middleware.ResponseWriterKey, rw)

		token := csrf.NewToken("session-123", []byte(tokenKey))
		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				TenantID:              tenantID,
				PostLogoutRedirectURI: postLogoutURL,
				Cookie:                "session-id-" + tenantID + "=session-123; csrf-token-" + tenantID + "=" + token,
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

func TestOpenAPIServer_ToErrorModel(t *testing.T) {
	server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})

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
				wantStatus: http.StatusBadRequest,
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
		server := newOpenAPIServer(mock, nil, "", "", []string{allowedBaseURL})

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
		server := newOpenAPIServer(mock, nil, "", "", []string{allowedBaseURL})

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
		sessionID := "session-123"
		tenantID := "tenant-1"

		mock := &mockSessionManager{
			logoutFunc: func(ctx context.Context, sid, postLogoutRedirectURL string) (string, error) {
				assert.Equal(t, sessionID, sid)
				assert.Equal(t, postLogoutURL, postLogoutRedirectURL)
				return expectedURL, nil
			},
		}
		server := newOpenAPIServer(mock, nil, "session-id", "csrf-token", []string{allowedBaseURL})

		rw := httptest.NewRecorder()
		ctx := context.WithValue(context.Background(), middleware.ResponseWriterKey, rw)

		logoutReq := openapi.LogoutRequestObject{
			Params: openapi.LogoutParams{
				TenantID:              tenantID,
				PostLogoutRedirectURI: postLogoutURL,
				Cookie:                "session-id-" + tenantID + "=" + sessionID + "; csrf-token-" + tenantID + "=csrf-token",
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

func Test_urlWithErrorCodeAndDescription(t *testing.T) {
	t.Run("absolute with hash based routing", func(t *testing.T) {
		result := urlWithErrorCodeAndDescription("https://my.domain/#/foo/bar", "code", "desc")
		assert.Equal(t, "https://my.domain/#/foo/bar?errorCode=code&errorDescription=desc", result)
	})
	t.Run("absolute without hash based routing", func(t *testing.T) {
		result := urlWithErrorCodeAndDescription("https://my.domain/foo/bar", "code", "desc")
		assert.Equal(t, "https://my.domain/foo/bar?errorCode=code&errorDescription=desc", result)
	})
	t.Run("relative with hash based routing", func(t *testing.T) {
		result := urlWithErrorCodeAndDescription("/#/foo/bar", "code", "desc")
		assert.Equal(t, "/#/foo/bar?errorCode=code&errorDescription=desc", result)
	})
	t.Run("relative without hash based routing", func(t *testing.T) {
		result := urlWithErrorCodeAndDescription("/foo/bar", "code", "desc")
		assert.Equal(t, "/foo/bar?errorCode=code&errorDescription=desc", result)
	})
	t.Run("invalid URL", func(t *testing.T) {
		result := urlWithErrorCodeAndDescription("ht!ps://my.domain/foo/bar", "code", "desc")
		assert.Empty(t, result)
	})
	t.Run("empty fragment", func(t *testing.T) {
		result := urlWithErrorCodeAndDescription("/foo/bar#", "code", "desc")
		assert.Equal(t, "/foo/bar?errorCode=code&errorDescription=desc", result)
	})
}

func TestOpenAPIServer_BuildErrorRedirectURL(t *testing.T) {
	ctx := t.Context()
	server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})

	t.Run("returns empty when errorURI is empty", func(t *testing.T) {
		result := server.buildErrorRedirectURL(ctx, "", serviceerr.ErrNotFound)
		assert.Empty(t, result)
	})

	t.Run("returns empty when errorURI is not in allowed list", func(t *testing.T) {
		result := server.buildErrorRedirectURL(ctx, "https://evil.com/error", serviceerr.ErrNotFound)
		assert.Empty(t, result)
	})

	t.Run("returns redirect URL with error code for allowed errorURI", func(t *testing.T) {
		result := server.buildErrorRedirectURL(ctx, "https://app.example.com/error", serviceerr.ErrNotFound)
		assert.Contains(t, result, "errorCode=not_found")
		assert.Contains(t, result, "errorDescription=not+found")
	})

	t.Run("preserves existing query params and adds errorCode", func(t *testing.T) {
		result := server.buildErrorRedirectURL(ctx, "https://app.example.com/error?tenant=x", serviceerr.ErrStateExpired)
		assert.Contains(t, result, "errorCode=state_expired")
		assert.Contains(t, result, "tenant=x")
		assert.Contains(t, result, "https://app.example.com/error?")
	})

	t.Run("maps unknown error to unknown", func(t *testing.T) {
		result := server.buildErrorRedirectURL(ctx, "https://app.example.com/error", errors.New("random"))
		assert.Contains(t, result, "errorCode=unknown")
		assert.Contains(t, result, "errorDescription=unknown+error")
	})

	t.Run("handles access denied error", func(t *testing.T) {
		result := server.buildErrorRedirectURL(ctx, "https://app.example.com/error", serviceerr.ErrAccessDenied)
		assert.Contains(t, result, "errorCode=access_denied")
	})
}

func TestOpenAPIServer_GetErrorURIFromState(t *testing.T) {
	ctx := t.Context()

	t.Run("returns empty when sManager is nil", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		result := server.getErrorURIFromState(ctx, "some-state")
		assert.Empty(t, result)
	})

	t.Run("returns empty when LoadState fails", func(t *testing.T) {
		mock := &mockSessionManager{
			loadStateFunc: func(ctx context.Context, stateID string) (session.State, error) {
				return session.State{}, errors.New("not found")
			},
		}
		server := newOpenAPIServer(mock, nil, "", "", []string{allowedBaseURL})
		result := server.getErrorURIFromState(ctx, "some-state")
		assert.Empty(t, result)
	})

	t.Run("returns errorURI from state", func(t *testing.T) {
		mock := &mockSessionManager{
			loadStateFunc: func(ctx context.Context, stateID string) (session.State, error) {
				return session.State{ErrorURI: "https://app.example.com/error"}, nil
			},
		}
		server := newOpenAPIServer(mock, nil, "", "", []string{allowedBaseURL})
		result := server.getErrorURIFromState(ctx, "some-state")
		assert.Equal(t, "https://app.example.com/error", result)
	})
}

func TestOpenAPIServer_AuthErrorResponse(t *testing.T) {
	ctx := t.Context()

	t.Run("returns JSON when no errorURI", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		resp := server.authErrorResponse(ctx, "", serviceerr.ErrUnknown)
		assert.IsType(t, openapi.AuthdefaultJSONResponse{}, resp)
	})

	t.Run("returns redirect when errorURI is valid", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		resp := server.authErrorResponse(ctx, "https://app.example.com/error", serviceerr.ErrNotFound)
		assert.IsType(t, openapi.Auth302Response{}, resp)
		r, ok := resp.(openapi.Auth302Response)
		require.True(t, ok)
		assert.Contains(t, r.Headers.Location, "errorCode=not_found")
	})
}

func TestOpenAPIServer_CallbackErrorResponse(t *testing.T) {
	ctx := t.Context()

	t.Run("returns JSON when no errorURI", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		resp := server.callbackErrorResponse(ctx, "", serviceerr.ErrUnknown)
		assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)
	})

	t.Run("returns redirect when errorURI is valid", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		resp := server.callbackErrorResponse(ctx, "https://app.example.com/error", serviceerr.ErrStateExpired)
		assert.IsType(t, openapi.Callback302Response{}, resp)
		r, ok := resp.(openapi.Callback302Response)
		require.True(t, ok)
		assert.Contains(t, r.Headers.Location, "errorCode=state_expired")
	})
}

func TestOpenAPIServer_CallbackFinaliseErrorResponse(t *testing.T) {
	ctx := t.Context()

	t.Run("returns redirect when errorURI is valid", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		resp := server.callbackFinaliseErrorResponse(ctx, "https://app.example.com/error", serviceerr.ErrAccessDenied)
		assert.IsType(t, openapi.Callback302Response{}, resp)
		_, ok := resp.(openapi.Callback302Response)
		require.True(t, ok)
	})

	t.Run("masks 403 to unauthorized when no errorURI", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		resp := server.callbackFinaliseErrorResponse(ctx, "", serviceerr.ErrAccessDenied)
		assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)
		r, ok := resp.(openapi.CallbackdefaultJSONResponse)
		require.True(t, ok)
		assert.Equal(t, string(serviceerr.CodeUnauthorizedClient), r.Body.Error)
		assert.Equal(t, http.StatusUnauthorized, r.StatusCode)
	})

	t.Run("returns original error when not 403 and no errorURI", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "", []string{allowedBaseURL})
		resp := server.callbackFinaliseErrorResponse(ctx, "", serviceerr.ErrStateExpired)
		assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)
		r, ok := resp.(openapi.CallbackdefaultJSONResponse)
		require.True(t, ok)
		assert.Equal(t, string(serviceerr.CodeStateExpired), r.Body.Error)
		assert.Equal(t, http.StatusGone, r.StatusCode)
	})
}

// Note: Auth and Callback functions have lower unit test coverage because they depend on
// HTTP middleware setup. These functions are more thoroughly tested in integration tests
// where the full HTTP middleware stack is available. The current unit tests cover:
// - Error handling when response writer is not in context
// - Error model conversion
// For full coverage of success paths and session manager interactions, see integration tests.
