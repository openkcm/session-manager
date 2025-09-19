package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/session-manager/internal/openapi"
	"github.com/openkcm/session-manager/internal/session"
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
	if request.Params.TenantID == nil {
		return nil, errors.New("missing tenant_id parameter")
	}

	if request.Params.RequestURI == nil {
		return nil, errors.New("missing request_uri parameter")
	}

	// TODO(Danylo): Make fingerprint
	url, err := s.sManager.Auth(ctx, *request.Params.TenantID, "fingerprint", *request.Params.RequestURI)
	if err != nil {
		return nil, fmt.Errorf("authenticating with session manaager: %w", err)
	}

	return openapi.Auth302Response{
		Headers: openapi.Auth302ResponseHeaders{
			Location: url,
		},
	}, nil
}
