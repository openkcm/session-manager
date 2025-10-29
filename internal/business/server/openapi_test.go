package server

import (
	"context"
	"net/http"
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
	assert.Equal(t, int(serviceerr.CodeUnknown), *r.Body.ErrorCode)
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
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
	assert.Equal(t, int(serviceerr.CodeUnknown), *r.Body.ErrorCode)
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
}
