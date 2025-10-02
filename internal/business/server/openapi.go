package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/session-manager/internal/openapi"
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
	// TODO(Danylo): Make fingerprint
	url, err := s.sManager.Auth(ctx, request.Params.TenantID, "fingerprint", request.Params.RequestURI)
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
	//TODO implement callback logic
	return nil, errors.New("callback not yet implemented")
}
