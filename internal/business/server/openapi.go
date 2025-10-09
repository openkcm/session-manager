package server

import (
	"context"
	"errors"
	"fmt"

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
		return nil, err
	}
	url, err := s.sManager.Auth(ctx, request.Params.TenantID, extractFingerprint, request.Params.RequestURI)
	if err != nil {
		return nil, fmt.Errorf("authenticating with session manager: %w", err)
	}

	return openapi.Auth302Response{
		Headers: openapi.Auth302ResponseHeaders{
			Location: url,
		},
	}, nil
}

// Callback implements openapi.StrictServerInterface.
func (s *openAPIServer) Callback(ctx context.Context, req openapi.CallbackRequestObject) (openapi.CallbackResponseObject, error) {
	var currentFingerprint string
	currentFingerprint, err := fingerprint.ExtractFingerprint(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.sManager.Callback(ctx, req.Params.State, req.Params.Code, currentFingerprint)

	if err != nil {
		if errors.Is(err, serviceerr.ErrFingerprintMismatch) {
			errorCode := 403
			errorMsg := "fingerprint mismatch"
			return openapi.Callback403JSONResponse{
				ErrorCode: &errorCode,
				ErrorMsg:  &errorMsg,
			}, nil
		}
		return nil, err
	}

	cookies := []string{
		fmt.Sprintf("__Host-Http-SESSION=%s; Path=/; Secure; HttpOnly; SameSite=Strict", result.SessionID),
		fmt.Sprintf("__Host-CSRF=%s; Path=/; Secure; SameSite=Strict", result.CSRFToken),
	}

	return openapi.Callback302Response{
		Headers: openapi.Callback302ResponseHeaders{
			Location:  result.RedirectURI,
			SetCookie: cookies,
		},
	}, nil
}
