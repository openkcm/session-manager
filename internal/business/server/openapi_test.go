package server

import (
	"context"
	"testing"

	"github.com/openkcm/session-manager/internal/openapi"
	"github.com/stretchr/testify/assert"
)

func TestOpenAPIServer_Auth_NilManager(t *testing.T) {
	server := newOpenAPIServer(nil)
	req := openapi.AuthRequestObject{
		Params: openapi.AuthParams{
			TenantID:   "tenant",
			RequestURI: "/uri",
		},
	}
	_, err := server.Auth(context.Background(), req)
	assert.Error(t, err)
}

func TestOpenAPIServer_Callback_NilManager(t *testing.T) {
	server := newOpenAPIServer(nil)
	req := openapi.CallbackRequestObject{
		Params: openapi.CallbackParams{
			State: "state",
			Code:  "code",
		},
	}
	_, err := server.Callback(context.Background(), req)
	assert.Error(t, err)
}

func TestOpenAPIServer_Auth_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := newOpenAPIServer(nil)
	req := openapi.AuthRequestObject{}
	_, err := server.Auth(ctx, req)
	assert.Error(t, err)
}

func TestOpenAPIServer_Callback_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := newOpenAPIServer(nil)
	req := openapi.CallbackRequestObject{}
	_, err := server.Callback(ctx, req)
	assert.Error(t, err)
}
