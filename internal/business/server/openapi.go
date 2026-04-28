package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/openkcm/common-sdk/pkg/csrf"
	"github.com/openkcm/common-sdk/pkg/fingerprint"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/middleware"
	"github.com/openkcm/session-manager/internal/openapi"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/session"
)

// sessionManager defines the interface for session management operations
// used by the OpenAPI server.
type sessionManager interface {
	MakeAuthURI(ctx context.Context, tenantID, fingerprint, requestURI string) (string, string, error)
	FinaliseOIDCLogin(ctx context.Context, state, code, fingerprint string) (session.OIDCSessionData, error)
	MakeSessionCookie(ctx context.Context, tenantID, sessionID string) (*http.Cookie, error)
	MakeCSRFCookie(ctx context.Context, tenantID, csrfToken string) (*http.Cookie, error)
	MakeLoginCSRFCookie(ctx context.Context, csrfToken string) (*http.Cookie, error)
	Logout(ctx context.Context, sessionID, postLogoutRedirectURL string) (string, error)
	BCLogout(ctx context.Context, logoutToken string) error
}

// openAPIServer is an implementation of the OpenAPI interface.
type openAPIServer struct {
	sManager sessionManager

	csrfSecret []byte

	sessionIDCookieNamePrefix string
	csrfTokenCookieNamePrefix string
	allowedRedirectBaseURLs   []string
}

// Ensure openAPIServer implements [openapi.StrictServerInterface]
var _ openapi.StrictServerInterface = (*openAPIServer)(nil)

// newOpenAPIServer creates a new implementation of the openapi.StrictServerInterface.
func newOpenAPIServer(
	sManager sessionManager,
	csrfSecret []byte,
	sessionIDCookieNamePrefix,
	csrfTokenCookieNamePrefix string,
	allowedRedirectBaseURLs []string,
) *openAPIServer {
	return &openAPIServer{
		sManager:                  sManager,
		csrfSecret:                csrfSecret,
		sessionIDCookieNamePrefix: sessionIDCookieNamePrefix,
		csrfTokenCookieNamePrefix: csrfTokenCookieNamePrefix,
		allowedRedirectBaseURLs:   allowedRedirectBaseURLs,
	}
}

// Auth implements openapi.StrictServerInterface.
func (s *openAPIServer) Auth(ctx context.Context, request openapi.AuthRequestObject) (openapi.AuthResponseObject, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "auth")
	defer span.End()

	slogctx.Debug(ctx, "Auth() called", "tenantId", request.Params.TenantID, "requestUri", request.Params.RequestURI)
	defer slogctx.Debug(ctx, "Auth() completed")

	if !s.isAllowedRedirectBaseURL(request.Params.RequestURI) {
		err := fmt.Errorf("request URI does not match an allowed redirect base URL: %s", request.Params.RequestURI)
		span.RecordError(err)
		span.SetStatus(codes.Error, "request URI does not match an allowed redirect base URL")
		slogctx.Error(ctx, "Request URI does not match an allowed redirect base URL", "requestURI", request.Params.RequestURI)

		body, status := s.toErrorModel(err)
		return openapi.AuthdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	fingerprint, err := fingerprint.ExtractFingerprint(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to extract fingerprint")
		slogctx.Error(ctx, "Failed to extract fingerprint", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.AuthdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	url, csrfToken, err := s.sManager.MakeAuthURI(ctx, request.Params.TenantID, fingerprint, request.Params.RequestURI)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build auth URI")
		slogctx.Error(ctx, "Failed build auth URI", "error", err)

		body, status := s.toErrorModel(err)
		return openapi.AuthdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}
	loginCsrfCookie, err := s.sManager.MakeLoginCSRFCookie(ctx, csrfToken)
	if err != nil {
		span.RecordError(err)
		slogctx.Error(ctx, "Failed to make CSRF cookie", "error", err)
		body, status := s.toErrorModel(err)
		return openapi.AuthdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	span.SetStatus(codes.Ok, "")
	return openapi.Auth302Response{
		Headers: openapi.Auth302ResponseHeaders{
			Location:  url,
			SetCookie: loginCsrfCookie.String(),
		},
	}, nil
}

// Callback implements openapi.StrictServerInterface.
func (s *openAPIServer) Callback(ctx context.Context, req openapi.CallbackRequestObject) (openapi.CallbackResponseObject, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "callback")
	defer span.End()

	slogctx.Debug(ctx, "Callback() called", "state", req.Params.State)
	defer slogctx.Debug(ctx, "Callback() completed")

	currentFingerprint, err := fingerprint.ExtractFingerprint(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed extract fingerprint")
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
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get response writer from context")
		slogctx.Error(ctx, "Failed to get response writer from context", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}
	if !csrf.Validate(req.Params.UnderscoreUnderscoreHostLoginCSRF, req.Params.State, s.csrfSecret) {
		err := serviceerr.ErrInvalidLoginCSRFToken
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		body, status := newBadRequest(err.Error())
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}
	result, err := s.sManager.FinaliseOIDCLogin(ctx, req.Params.State, req.Params.Code, currentFingerprint)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to finalise OIDC login")
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
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create session cookie")
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
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create CSRF cookie")
		slogctx.Error(ctx, "Failed to create CSRF cookie", "error", err)

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

	span.SetStatus(codes.Ok, "")
	slogctx.Debug(ctx, "Redirecting user", "to", result.RequestURI)
	return openapi.Callback302Response{
		Headers: openapi.Callback302ResponseHeaders{
			Location: result.RequestURI,
		},
	}, nil
}

// Logout implements openapi.StrictServerInterface.
func (s *openAPIServer) Logout(ctx context.Context, request openapi.LogoutRequestObject) (openapi.LogoutResponseObject, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "logout")
	defer span.End()

	slogctx.Debug(ctx, "Logout() called", "tenantId", request.Params.TenantID, "postLogoutRedirectURI", request.Params.PostLogoutRedirectURI)
	defer slogctx.Debug(ctx, "Logout() completed")

	if !s.isAllowedRedirectBaseURL(request.Params.PostLogoutRedirectURI) {
		err := fmt.Errorf("post logout redirect URI does not match an allowed redirect base URL: %s", request.Params.PostLogoutRedirectURI)
		span.RecordError(err)
		span.SetStatus(codes.Error, "post logout redirect URI does not match an allowed redirect base URL")
		slogctx.Error(ctx, "Post logout redirect URI does not match an allowed redirect base URL", "postLogoutRedirectURI", request.Params.PostLogoutRedirectURI)

		body, status := s.toErrorModel(err)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	rw, err := middleware.ResponseWriterFromContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get response writer from context")
		slogctx.Error(ctx, "Failed to get response writer from context", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	cookies, err := http.ParseCookie(request.Params.Cookie)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to parse 'Cookie' header")
		slogctx.Warn(ctx, "failed to parse 'Cookie' header", "error", err)

		body, status := newBadRequest("invalid 'Cookie' header")
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	sessionCookieName := s.sessionIDCookieNamePrefix + "-" + request.Params.TenantID
	csrfCookieName := s.csrfTokenCookieNamePrefix + "-" + request.Params.TenantID
	var sessionCookie *http.Cookie
	var cookiesToClear []*http.Cookie

	// http.ParseCookie limits the number of cookies to 3000
	// (configurable with $GODEBUG environment variable, see httpcookiemaxnum),
	// so we can safely iterate over the cookies.
	for _, cookie := range cookies {
		switch cookie.Name {
		case sessionCookieName:
			sessionCookie = cookie
			cookiesToClear = append(cookiesToClear, cookie)
		case csrfCookieName:
			cookiesToClear = append(cookiesToClear, cookie)
		}
		if len(cookiesToClear) == 2 {
			break
		}
	}

	if sessionCookie == nil || sessionCookie.Value == "" {
		body, status := newBadRequest("missing session id in the cookies")
		slogctx.Warn(ctx, "missing session id in the cookies")
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	logoutURL, err := s.sManager.Logout(ctx, sessionCookie.Value, request.Params.PostLogoutRedirectURI)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to logout user")
		slogctx.Error(ctx, "failed to logout user", "error", err)

		body, status := s.toErrorModel(err)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	// Reset all cookies
	for _, cookie := range cookiesToClear {
		cookie.MaxAge = -1
		cookie.Value = ""
		http.SetCookie(rw, cookie)
	}

	span.SetStatus(codes.Ok, "")
	return openapi.Logout302Response{
		Headers: openapi.Logout302ResponseHeaders{
			Location: logoutURL,
		},
	}, nil
}

func (s *openAPIServer) Bclogout(ctx context.Context, request openapi.BclogoutRequestObject) (openapi.BclogoutResponseObject, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "bc_logout")
	defer span.End()

	slogctx.Debug(ctx, "Bclogout() called")
	defer slogctx.Debug(ctx, "Bclogout() completed")

	if err := s.sManager.BCLogout(ctx, request.Body.LogoutToken); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "back channel logout failed")
		slogctx.Error(ctx, "back-channel logout failed", "error", err)
		body, _ := s.toErrorModel(err)
		return openapi.Bclogout400JSONResponse(body), nil
	}

	span.SetStatus(codes.Ok, "")
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

func (s *openAPIServer) isAllowedRedirectBaseURL(url string) bool {
	for _, baseURL := range s.allowedRedirectBaseURLs {
		if strings.HasPrefix(url, baseURL) {
			return true
		}
	}
	return false
}

func newBadRequest(description string) (model openapi.ErrorModel, httpStatus int) {
	return openapi.ErrorModel{
		Error:            string(serviceerr.CodeInvalidRequest),
		ErrorDescription: &description,
	}, http.StatusBadRequest
}
