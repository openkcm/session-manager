package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/openapi"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

func TestOpenAPIServer_Auth_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := newOpenAPIServer(nil)
	req := openapi.AuthRequestObject{}
	resp, err := server.Auth(ctx, req)
	assert.NoError(t, err)

	assert.IsType(t, openapi.AuthdefaultJSONResponse{}, resp)

	// Already asserted above
	r, _ := resp.(openapi.AuthdefaultJSONResponse)
	assert.Equal(t, int(serviceerr.CodeUnauthorized), *r.Body.ErrorCode)
	assert.Equal(t, int(serviceerr.CodeUnauthorized), r.StatusCode)
}

func TestOpenAPIServer_Callback_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := newOpenAPIServer(nil)
	req := openapi.CallbackRequestObject{}
	resp, err := server.Callback(ctx, req)
	assert.NoError(t, err)

	assert.IsType(t, openapi.CallbackdefaultJSONResponse{}, resp)

	// Already asserted above
	r, _ := resp.(openapi.CallbackdefaultJSONResponse)
	assert.Equal(t, int(serviceerr.CodeUnauthorized), *r.Body.ErrorCode)
	assert.Equal(t, int(serviceerr.CodeUnauthorized), r.StatusCode)
}
