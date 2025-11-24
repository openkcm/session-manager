package server

import (
	"context"
	"errors"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/middleware/responsewriter"
	"github.com/openkcm/session-manager/internal/openapi"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/pkg/fingerprint"
	"github.com/openkcm/session-manager/pkg/session"
)

// openAPIServer is an implementation of the OpenAPI interface.
type openAPIServer struct {
	sManager *session.Manager
}

// Ensure openAPIServer implements [openapi.StrictServerInterface]
var _ openapi.StrictServerInterface = (*openAPIServer)(nil)

// newOpenAPIServer creates a new implementation of the openapi.StrictServerInterface.
func newOpenAPIServer(sManager *session.Manager) *openAPIServer {
	return &openAPIServer{
		sManager: sManager,
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
	rw, err := responsewriter.ResponseWriterFromContext(ctx)
	if err != nil {
		slogctx.Error(ctx, "Failed to get response writer from context", "error", err)

		body, status := s.toErrorModel(serviceerr.ErrUnknown)
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	// Get CSRF cookie domain
	csrfCookieDomain, err := s.sManager.MakeCSRFCookieDomain()
	if err != nil {
		slogctx.Error(ctx, "Failed to make CSRF cookie domain", "error", err)

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
			return openapi.Callback403JSONResponse(body), nil
		}

		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: status,
		}, nil
	}

	// Session cookie
	sessionCookie := &http.Cookie{
		Name:     "__Host-Http-SESSION",
		Value:    result.SessionID,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		HttpOnly: true,
	}

	// CSRF cookie
	csrfCookie := &http.Cookie{
		Name:     "CSRF",
		Value:    result.CSRFToken,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Domain:   csrfCookieDomain,
	}

	// There is a limitation of OpenAPI that does not allow setting multiple cookies
	// with the strict handlers. Therefore, we do not define the Set-Cookie header
	// in the yaml spec. However, in the actual implementation both cookies are set.
	// See https://github.com/OAI/OpenAPI-Specification/issues/1237 for details.
	http.SetCookie(rw, sessionCookie)
	http.SetCookie(rw, csrfCookie) // NOSONAR

	slogctx.Debug(ctx, "Redirecting user", "to", result.RequestURI)
	return openapi.Callback302Response{
		Headers: openapi.Callback302ResponseHeaders{
			Location: result.RequestURI,
		},
	}, nil
}

func (s *openAPIServer) toErrorModel(err error) (model openapi.ErrorModel, httpStatus int) {
	var serviceErr *serviceerr.Error
	if !errors.As(err, &serviceErr) {
		serviceErr = serviceerr.ErrUnknown
	}

	return openapi.ErrorModel{
		ErrorCode: (*int)(&serviceErr.Code),
		ErrorMsg:  &serviceErr.Message,
	}, serviceErr.HTTPStatus()
}
