package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"

	"github.com/openkcm/common-sdk/pkg/csrf"
	"github.com/openkcm/common-sdk/pkg/fingerprint"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/middleware"
	"github.com/openkcm/session-manager/internal/openapi"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/session"
)

// openAPIServer is an implementation of the OpenAPI interface.
type openAPIServer struct {
	sManager *session.Manager

	csrfSecret []byte

	sessionIDCookieName,
	csrfTokenCookieName string
}

// Ensure openAPIServer implements [openapi.StrictServerInterface]
var _ openapi.StrictServerInterface = (*openAPIServer)(nil)

// newOpenAPIServer creates a new implementation of the openapi.StrictServerInterface.
func newOpenAPIServer(
	sManager *session.Manager,
	csrfSecret []byte,
	sessionIDCookieName,
	csrfTokenCookieName string,
) *openAPIServer {
	return &openAPIServer{
		sManager:            sManager,
		csrfSecret:          csrfSecret,
		sessionIDCookieName: sessionIDCookieName,
		csrfTokenCookieName: csrfTokenCookieName,
	}
}

// Auth implements openapi.StrictServerInterface.
func (s *openAPIServer) Auth(ctx context.Context, request openapi.AuthRequestObject) (openapi.AuthResponseObject, error) {
	slogctx.Debug(ctx, "Auth() called", "tenant_id", request.Params.TenantID, "request_uri", request.Params.RequestURI)
	defer slogctx.Debug(ctx, "Auth() completed")

	var extractFingerprint string
	extractFingerprint, err := fingerprint.ExtractFingerprint(ctx)
	if err != nil {
		slogctx.Error(ctx, "Failed to extract fingerprint", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.AuthdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	url, err := s.sManager.MakeAuthURI(ctx, request.Params.TenantID, extractFingerprint, request.Params.RequestURI)
	if err != nil {
		slogctx.Error(ctx, "Failed build auth URI", "error", err)

		body, status := s.toErrorModel(err)
		return openapi.AuthdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	return openapi.Auth302Response{
		Headers: openapi.Auth302ResponseHeaders{
			Location: url,
		},
	}, nil
}

// Callback implements openapi.StrictServerInterface.
func (s *openAPIServer) Callback(ctx context.Context, req openapi.CallbackRequestObject) (openapi.CallbackResponseObject, error) {
	slogctx.Debug(ctx, "Callback() called", "state", req.Params.State)
	defer slogctx.Debug(ctx, "Callback() completed")

	currentFingerprint, err := fingerprint.ExtractFingerprint(ctx)
	if err != nil {
		slogctx.Error(ctx, "Failed to extract fingerprint", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	// Get the response writer from the context
	rw, err := middleware.ResponseWriterFromContext(ctx)
	if err != nil {
		slogctx.Error(ctx, "Failed to get response writer from context", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	result, err := s.sManager.FinaliseOIDCLogin(ctx, req.Params.State, req.Params.Code, currentFingerprint)
	if err != nil {
		slogctx.Error(ctx, "Failed to finalise OIDC login", "error", err)

		body, status := s.toErrorModel(err)
		if status == 403 {
			// return generic Unauthorized for 403 Forbidden to avoid leaking information on
			// fingerprint mismatch in the original error body
			body, status = s.toErrorModel(serviceerr.ErrUnauthorized)
		}

		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	// Session cookie
	sessionCookie, err := s.sManager.MakeSessionCookie(ctx, result.TenantID, result.SessionID)
	if err != nil {
		slogctx.Error(ctx, "Failed to create session cookie", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	// CSRF cookie
	csrfCookie, err := s.sManager.MakeCSRFCookie(ctx, result.TenantID, result.CSRFToken)
	if err != nil {
		slogctx.Error(ctx, "Failed to create CSRF cookie", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	// Create old cookies without tenant suffix for backward compatibility
	// TODO: Remove these cookies as soon as UI and ExtAuthZ use the new cookies with tenant suffix
	oldSessionCookie, err := s.sManager.MakeSessionCookie(ctx, "", result.SessionID)
	if err != nil {
		slogctx.Error(ctx, "Failed to create old session cookie", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}
	oldCsrfCookie, err := s.sManager.MakeCSRFCookie(ctx, "", result.CSRFToken)
	if err != nil {
		slogctx.Error(ctx, "Failed to create old CSRF cookie", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	// There is a limitation of OpenAPI that does not allow setting multiple cookies
	// with the strict handlers. Therefore, we do not define the Set-Cookie header
	// in the yaml spec. However, in the actual implementation both cookies are set.
	// See https://github.com/OAI/OpenAPI-Specification/issues/1237 for details.
	http.SetCookie(rw, sessionCookie)
	http.SetCookie(rw, csrfCookie)
	// Set old cookies without tenant suffix for backward compatibility
	// TODO: Remove these cookies as soon as UI and ExtAuthZ use the new cookies with tenant suffix
	http.SetCookie(rw, oldSessionCookie)
	http.SetCookie(rw, oldCsrfCookie)

	slogctx.Debug(ctx, "Redirecting user", "to", result.RequestURI)
	return openapi.Callback302Response{
		Headers: openapi.Callback302ResponseHeaders{
			Location: result.RequestURI,
		},
	}, nil
}

// Logout implements openapi.StrictServerInterface.
func (s *openAPIServer) Logout(ctx context.Context, request openapi.LogoutRequestObject) (openapi.LogoutResponseObject, error) {
	slogctx.Debug(ctx, "Logout() called")
	defer slogctx.Debug(ctx, "Logout() completed")

	rw, err := middleware.ResponseWriterFromContext(ctx)
	if err != nil {
		slogctx.Error(ctx, "Failed to get response writer from context", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	cookies, err := http.ParseCookie(request.Params.Cookie)
	if err != nil {
		slogctx.Warn(ctx, "failed to parse 'Cookie' header", "error", err)

		body, status := newBadRequest("invalid 'Cookie' header")
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	var sessionCookie *http.Cookie
	var csrfCookie *http.Cookie

	// http.ParseCookie limits the number of cookies to 3000
	// (configurable with $GODEBUG environment variable, see httpcookiemaxnum),
	// so we can safely iterate over the cookies.
	for _, cookie := range cookies {
		switch cookie.Name {
		case s.csrfTokenCookieName:
			csrfCookie = cookie
		case s.sessionIDCookieName:
			sessionCookie = cookie
		}

		if sessionCookie.Value != "" && csrfCookie.Value != "" {
			break
		}
	}

	if sessionCookie.Value == "" {
		body, status := newBadRequest("missing session id in the cookies")
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	if csrfCookie.Value == "" {
		body, status := newBadRequest("missing csrf token in the cookies")
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	if !csrf.Validate(request.Params.XCSRFToken, sessionCookie.Value, s.csrfSecret) {
		csrfTokenHash := sha256.New().Sum([]byte(csrfCookie.Value))
		csrfSecretHash := sha256.New().Sum(s.csrfSecret)
		sessionIDHash := sha256.New().Sum([]byte(sessionCookie.Value))

		slogctx.Warn(ctx, "received invalid csrf token value", "csrf_token_hash", csrfTokenHash[:5], "csrf_secret_hash", csrfSecretHash[:5], "session_id_hash", sessionIDHash[:5])

		body, status := s.toErrorModel(serviceerr.ErrInvalidCSRFToken)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	logoutURL, err := s.sManager.Logout(ctx, sessionCookie.Value)
	if err != nil {
		slogctx.Error(ctx, "failed to logout user", "error", err)

		body, status := s.toErrorModel(err)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	// Reset all cookies
	for _, cookie := range []*http.Cookie{csrfCookie, sessionCookie} {
		cookie.MaxAge = -1
		cookie.Value = ""
		http.SetCookie(rw, cookie)
	}

	return openapi.Logout302Response{
		Headers: openapi.Logout302ResponseHeaders{
			Location: logoutURL,
		},
	}, nil
}

func (s *openAPIServer) Bclogout(ctx context.Context, request openapi.BclogoutRequestObject) (openapi.BclogoutResponseObject, error) {
	slogctx.Debug(ctx, "Bclogout() called")
	defer slogctx.Debug(ctx, "Bclogout() completed")

	if err := s.sManager.BCLogout(ctx, request.Body.LogoutToken); err != nil {
		body, _ := s.toErrorModel(err)
		return openapi.Bclogout400JSONResponse(body), nil
	}

	return openapi.Bclogout200Response{}, nil
}

func (s *openAPIServer) toErrorModel(err error) (model openapi.ErrorModel, httpStatus int) {
	var serviceErr *serviceerr.Error
	if !errors.As(err, &serviceErr) {
		serviceErr = serviceerr.ErrUnknown
	}

	return openapi.ErrorModel{
		Error:            string(serviceErr.Err),
		ErrorDescription: &serviceErr.Description,
	}, serviceErr.HTTPStatus()
}

func newBadRequest(description string) (model openapi.ErrorModel, httpStatus int) {
	return openapi.ErrorModel{
		Error:            string(serviceerr.CodeInvalidRequest),
		ErrorDescription: &description,
	}, http.StatusBadRequest
}
