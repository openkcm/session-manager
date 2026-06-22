package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/openkcm/common-sdk/pkg/csrf"
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
	MakeAuthURI(ctx context.Context, tenantID, requestURI, errorURI string) (string, string, error)
	FinaliseOIDCLogin(ctx context.Context, state, code string) (session.OIDCSessionData, error)
	MakeSessionCookie(ctx context.Context, tenantID, sessionID string) (*http.Cookie, error)
	MakeCSRFCookie(ctx context.Context, tenantID, csrfToken string) (*http.Cookie, error)
	MakeLoginCSRFCookie(ctx context.Context, csrfToken string) (*http.Cookie, error)
	LoadState(ctx context.Context, stateID string) (session.State, error)
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

	// Extract error_uri (optional, for backward compatibility with old UI that doesn't send it)
	errorURI := ""
	if request.Params.ErrorURI != nil {
		errorURI = *request.Params.ErrorURI
	}

	if !s.isAllowedRedirectBaseURL(request.Params.RequestURI) {
		svcerr := &serviceerr.Error{
			Err:         serviceerr.CodeInvalidRequest,
			Description: "request URI does not match an allowed redirect base URL",
		}
		serviceerr.RecordAndLogError(ctx, span, svcerr, "requestURI", request.Params.RequestURI)
		return s.authErrorResponse(ctx, errorURI, svcerr), nil
	}

	url, csrfToken, err := s.sManager.MakeAuthURI(ctx, request.Params.TenantID, request.Params.RequestURI, errorURI)
	if err != nil {
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
		return s.authErrorResponse(ctx, errorURI, err), nil
	}

	loginCsrfCookie, err := s.sManager.MakeLoginCSRFCookie(ctx, csrfToken)
	if err != nil {
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
		return s.authErrorResponse(ctx, errorURI, err), nil
	}

	span.SetStatus(codes.Ok, "")
	return openapi.Auth302Response{
		Headers: openapi.Auth302ResponseHeaders{
			Location:  url,
			SetCookie: loginCsrfCookie.String(),
		},
	}, nil
}

// authErrorResponse returns either a redirect to the error page or a JSON error response for the Auth endpoint.
func (s *openAPIServer) authErrorResponse(ctx context.Context, errorURI string, err error) openapi.AuthResponseObject {
	if redirectURL := s.buildErrorRedirectURL(ctx, errorURI, err); redirectURL != "" {
		return openapi.Auth302Response{
			Headers: openapi.Auth302ResponseHeaders{Location: redirectURL},
		}
	}
	body, status := s.toErrorModel(err)
	return openapi.AuthdefaultJSONResponse{
		Body:       body,
		StatusCode: status,
	}
}

// Callback implements openapi.StrictServerInterface.
func (s *openAPIServer) Callback(ctx context.Context, req openapi.CallbackRequestObject) (openapi.CallbackResponseObject, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "callback")
	defer span.End()

	slogctx.Debug(ctx, "Callback() called", "state", req.Params.State)
	defer slogctx.Debug(ctx, "Callback() completed")

	// Try to load error_uri from state (best-effort, for error redirect)
	errorURI := s.getErrorURIFromState(ctx, req.Params.State)

	// Get the response writer from the context
	rw, err := middleware.ResponseWriterFromContext(ctx)
	if err != nil {
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
		return s.callbackErrorResponse(ctx, errorURI, serviceerr.ErrUnknown), nil
	}

	if !csrf.Validate(req.Params.UnderscoreUnderscoreHostLoginCSRF, req.Params.State, s.csrfSecret) {
		svcerr := serviceerr.ErrInvalidLoginCSRFToken
		serviceerr.RecordAndLogError(ctx, span, svcerr)
		return s.callbackErrorResponse(ctx, errorURI, svcerr), nil
	}

	result, err := s.sManager.FinaliseOIDCLogin(ctx, req.Params.State, req.Params.Code)
	if err != nil {
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
		return s.callbackFinaliseErrorResponse(ctx, errorURI, err), nil
	}

	// Session cookie
	sessionCookie, err := s.sManager.MakeSessionCookie(ctx, result.TenantID, result.SessionID)
	if err != nil {
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
		return s.callbackErrorResponse(ctx, result.ErrorURI, serviceerr.ErrUnknown), nil
	}

	// CSRF cookie
	csrfCookie, err := s.sManager.MakeCSRFCookie(ctx, result.TenantID, result.CSRFToken)
	if err != nil {
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
		return s.callbackErrorResponse(ctx, result.ErrorURI, serviceerr.ErrUnknown), nil
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

// callbackErrorResponse returns either a redirect to the error page or a JSON error response.
func (s *openAPIServer) callbackErrorResponse(ctx context.Context, errorURI string, err error) openapi.CallbackResponseObject {
	if redirectURL := s.buildErrorRedirectURL(ctx, errorURI, err); redirectURL != "" {
		return openapi.Callback302Response{
			Headers: openapi.Callback302ResponseHeaders{Location: redirectURL},
		}
	}
	body, status := s.toErrorModel(err)
	return openapi.CallbackdefaultJSONResponse{
		Body:       body,
		StatusCode: status,
	}
}

// callbackFinaliseErrorResponse handles the error case after FinaliseOIDCLogin fails,
// masking sensitive details when no error redirect is available.
func (s *openAPIServer) callbackFinaliseErrorResponse(ctx context.Context, errorURI string, err error) openapi.CallbackResponseObject {
	if redirectURL := s.buildErrorRedirectURL(ctx, errorURI, err); redirectURL != "" {
		return openapi.Callback302Response{
			Headers: openapi.Callback302ResponseHeaders{Location: redirectURL},
		}
	}

	body, status := s.toErrorModel(err)
	if status == 403 {
		// return generic Unauthorized for 403 Forbidden to avoid leaking sensitive information
		body, status = s.toErrorModel(serviceerr.ErrUnauthorized)
	}
	return openapi.CallbackdefaultJSONResponse{
		Body:       body,
		StatusCode: status,
	}
}

// Logout implements openapi.StrictServerInterface.
func (s *openAPIServer) Logout(ctx context.Context, request openapi.LogoutRequestObject) (openapi.LogoutResponseObject, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "logout")
	defer span.End()

	slogctx.Debug(ctx, "Logout() called", "tenantId", request.Params.TenantID, "postLogoutRedirectURI", request.Params.PostLogoutRedirectURI)
	defer slogctx.Debug(ctx, "Logout() completed")

	if !s.isAllowedRedirectBaseURL(request.Params.PostLogoutRedirectURI) {
		svcerr := &serviceerr.Error{
			Err:         serviceerr.CodeInvalidRequest,
			Description: "post logout redirect URI does not match an allowed redirect base URL",
		}
		serviceerr.RecordAndLogError(ctx, span, svcerr, "postLogoutRedirectURI", request.Params.PostLogoutRedirectURI)
		body, status := s.toErrorModel(svcerr)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	rw, err := middleware.ResponseWriterFromContext(ctx)
	if err != nil {
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	cookies, err := http.ParseCookie(request.Params.Cookie)
	if err != nil {
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
		body, status := newBadRequest("invalid 'Cookie' header")
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	sessionCookieValue := ""
	sessionCookieName := s.sessionIDCookieNamePrefix + "-" + request.Params.TenantID
	csrfCookieName := s.csrfTokenCookieNamePrefix + "-" + request.Params.TenantID
	var cookiesToClear []*http.Cookie

	// http.ParseCookie limits the number of cookies to 3000
	// (configurable with $GODEBUG environment variable, see httpcookiemaxnum),
	// so we can safely iterate over the cookies.
	for _, cookie := range cookies {
		// Stop iterating once we have found both cookies to clear, to avoid unnecessary iterations
		if len(cookiesToClear) == 2 {
			break
		}
		// We only care about the session cookie and the CSRF cookie for the current tenant, so skip any other cookies
		if cookie.Name != sessionCookieName && cookie.Name != csrfCookieName {
			continue
		}
		// We need the session cookie value to perform the logout, so keep a reference to it
		if cookie.Name == sessionCookieName {
			sessionCookieValue = cookie.Value
		}
		// To clear a cookie, we set its MaxAge to -1 and Value to an empty string
		cookie.MaxAge = -1
		cookie.Value = ""
		// Mitigate CxONE findings around missing security flags on cookies,
		// even though these cookies are being deleted - set the flags to be safe
		cookie.Secure = true
		cookie.SameSite = http.SameSiteStrictMode
		cookie.HttpOnly = true
		// Add the cookie to the list of cookies to clear after successful logout
		cookiesToClear = append(cookiesToClear, cookie)
	}

	if sessionCookieValue == "" {
		svcerr := &serviceerr.Error{
			Err:         serviceerr.CodeInvalidRequest,
			Description: "missing session id in the cookies",
		}
		serviceerr.RecordAndLogError(ctx, span, svcerr)
		body, status := s.toErrorModel(svcerr)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	logoutURL, err := s.sManager.Logout(ctx, sessionCookieValue, request.Params.PostLogoutRedirectURI)
	if err != nil {
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
		body, status := s.toErrorModel(err)
		return openapi.LogoutdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	for _, cookie := range cookiesToClear {
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
		serviceerr.RecordAndLogError(ctx, span, err, "error", err)
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

// buildErrorRedirectURL constructs a redirect URL to the UI error page with the error code.
// Returns empty string if errorURI is not provided or not allowed (backward-compatible fallback to JSON).
func (s *openAPIServer) buildErrorRedirectURL(ctx context.Context, errorURI string, err error) string {
	if errorURI == "" {
		return ""
	}

	// Validate error_uri against allowed redirect base URLs to prevent open redirect
	if !s.isAllowedRedirectBaseURL(errorURI) {
		return ""
	}

	var serviceErr *serviceerr.Error
	if !errors.As(err, &serviceErr) {
		serviceErr = serviceerr.ErrUnknown
	}

	errorCode := serviceErr.Err

	redirect := urlWithErrorCodeAndDescription(errorURI, string(errorCode), serviceErr.Description)
	slogctx.Error(ctx, "Redirecting to error page",
		"errorCode", string(errorCode),
		"originalError", err.Error(),
		"errorURI", errorURI,
		"redirect", redirect,
	)
	return redirect
}

func urlWithErrorCodeAndDescription(s, code, desc string) string {
	if s == "" {
		return ""
	}

	// Parse the string as URL
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}

	// If it has a fragment, try to add error code and description there.
	// This is for SPA # based routing e.g. https://my.domain/#/errorpage
	f := urlWithErrorCodeAndDescription(u.Fragment, code, desc)
	if f != "" {
		u.Fragment = f
		return u.String()
	}

	// Otherwise add error code and description to the URL itself
	q := u.Query()
	q.Set("errorCode", code)
	q.Set("errorDescription", desc)
	u.RawQuery = q.Encode()
	return u.String()
}

// getErrorURIFromState loads the error_uri from state storage (best-effort).
// Returns empty string if state cannot be loaded.
func (s *openAPIServer) getErrorURIFromState(ctx context.Context, stateID string) string {
	if s.sManager == nil {
		return ""
	}
	state, err := s.sManager.LoadState(ctx, stateID)
	if err != nil {
		return ""
	}
	return state.ErrorURI
}
