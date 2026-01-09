package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/middleware"
	"github.com/openkcm/session-manager/internal/openapi"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

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

func TestOpenAPIServer_Bclogout(t *testing.T) {
	t.Run("returns 400 with nil manager", func(t *testing.T) {
		server := newOpenAPIServer(nil, nil, "", "")
		ctx := t.Context()

		bclogoutReq := openapi.BclogoutRequestObject{
			Body: &openapi.BclogoutFormdataRequestBody{
				LogoutToken: "some-token",
			},
		}

		resp, err := server.Bclogout(ctx, bclogoutReq)

		require.NoError(t, err)
		// With nil manager, it should panic/fail
		assert.IsType(t, openapi.Bclogout400JSONResponse{}, resp)
	})
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
