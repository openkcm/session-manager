package server

import (
	"context"
	"errors"
	"fmt"

	slogctx "github.com/veqryn/slog-context"

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
	var extractFingerprint string
	extractFingerprint, err := fingerprint.ExtractFingerprint(ctx)
	if err != nil {
		message, code := s.toErrorModel(serviceerr.ErrUnknown)
		slogctx.Error(ctx, message, "error", code)

		unathorisedMsg := "Unauthorized"
		unauthorisedCode := 401
		return openapi.AuthdefaultJSONResponse{
			Body: openapi.ErrorModel{
				ErrorCode: &unauthorisedCode,
				ErrorMsg:  &unathorisedMsg,
			},
			StatusCode: 401,
		}, nil
	}

	url, err := s.sManager.MakeAuthURI(ctx, request.Params.TenantID, extractFingerprint, request.Params.RequestURI)
	if err != nil {
		message, code := s.toErrorModel(err)
		slogctx.Error(ctx, message, "error", code)

		unathorisedMsg := "Unauthorized"
		unauthorisedCode := 401
		return openapi.AuthdefaultJSONResponse{
			Body: openapi.ErrorModel{
				ErrorCode: &unauthorisedCode,
				ErrorMsg:  &unathorisedMsg,
			},
			StatusCode: 401,
		}, nil
	}

	slogctx.Info(ctx, "Redirecting user to the OIDC provider authentication URL")

	return openapi.Auth302Response{
		Headers: openapi.Auth302ResponseHeaders{
			Location: url,
		},
	}, nil
}

// Callback implements openapi.StrictServerInterface.
func (s *openAPIServer) Callback(ctx context.Context, req openapi.CallbackRequestObject) (openapi.CallbackResponseObject, error) {
	slogctx.Info(ctx, "Finalising OIDC flow")

	var currentFingerprint string
	currentFingerprint, err := fingerprint.ExtractFingerprint(ctx)
	if err != nil {
		message, code := s.toErrorModel(serviceerr.ErrUnknown)
		body := openapi.ErrorModel{
			ErrorCode: &code,
			ErrorMsg:  &message,
		}
		slogctx.Error(ctx, message, "error", code)
		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: code,
		}, nil
	}

	result, err := s.sManager.FinaliseOIDCLogin(ctx, req.Params.State, req.Params.Code, currentFingerprint)
	if err != nil {
		message, code := s.toErrorModel(err)
		slogctx.Error(ctx, message, "error", code)
		body := openapi.ErrorModel{
			ErrorCode: &code,
			ErrorMsg:  &message,
		}
		if code == 403 {
			return openapi.Callback403JSONResponse(body), nil
		}

		return openapi.CallbackdefaultJSONResponse{
			Body:       body,
			StatusCode: code,
		}, nil
	}

	cookies := []string{
		fmt.Sprintf("__Host-Http-SESSION=%s; Path=/; Secure; HttpOnly; SameSite=Strict", result.SessionID),
		fmt.Sprintf("__Host-CSRF=%s; Path=/; Secure; SameSite=Strict", result.CSRFToken),
	}

	slogctx.Info(ctx, "Redirecting user to the request URI", "request_uri", result.RequestURI)

	return openapi.Callback302Response{
		Headers: openapi.Callback302ResponseHeaders{
			Location:  result.RequestURI,
			SetCookie: cookies,
		},
	}, nil
}

func (s *openAPIServer) toErrorModel(err error) (string, int) {
	var serviceErr *serviceerr.Error
	if !errors.As(err, &serviceErr) {
		serviceErr = serviceerr.ErrUnknown
	}

	model := openapi.ErrorModel{
		ErrorCode: (*int)(&serviceErr.Code),
		ErrorMsg:  &serviceErr.Message,
	}
	return *model.ErrorMsg, *model.ErrorCode
}
