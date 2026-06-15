package server

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/config"
)

func TestStartHTTPServer_ContextCancellation(t *testing.T) {
	t.Run("gracefully shuts down when context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
			HTTP: config.HTTPServer{
				Address:         "localhost:0", // Use port 0 to get a random available port
				ShutdownTimeout: 1 * time.Second,
			},
			SessionManager: config.SessionManager{
				CSRFSecretParsed: make([]byte, 32), // Minimum required length
			},
		}

		// Start the server in a goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- StartHTTPServer(ctx, cfg, nil)
		}()

		// Give the server a moment to start
		time.Sleep(100 * time.Millisecond)

		// Cancel the context to trigger shutdown
		cancel()

		// Wait for shutdown to complete
		select {
		case err := <-errChan:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Server did not shut down within timeout")
		}
	})
}

func TestCreateHTTPServer(t *testing.T) {
	t.Run("creates HTTP server with default config", func(t *testing.T) {
		ctx := t.Context()
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
			HTTP: config.HTTPServer{
				Address: "localhost:8080",
			},
			SessionManager: config.SessionManager{
				CSRFSecretParsed: make([]byte, 32),
			},
		}

		server, err := createHTTPServer(ctx, cfg, nil)

		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, "localhost:8080", server.Addr)
		assert.NotNil(t, server.Handler)
	})

	t.Run("creates HTTP server with unix socket", func(t *testing.T) {
		ctx := t.Context()
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
			HTTP: config.HTTPServer{
				Address: "unix:///tmp/test.sock",
			},
			SessionManager: config.SessionManager{
				CSRFSecretParsed: make([]byte, 32),
			},
		}

		server, err := createHTTPServer(ctx, cfg, nil)

		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, "unix:///tmp/test.sock", server.Addr)
	})
}

func TestProcessHTTPServerError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantErr     bool
		errContains string
	}{
		{
			name:    "nil error returns nil",
			err:     nil,
			wantErr: false,
		},
		{
			name:    "ErrServerClosed returns nil",
			err:     http.ErrServerClosed,
			wantErr: false,
		},
		{
			name:        "other error returns wrapped error",
			err:         errors.New("connection refused"),
			wantErr:     true,
			errContains: "HTTP server failed",
		},
		{
			name:        "wrapped error preserves original",
			err:         errors.New("bind address already in use"),
			wantErr:     true,
			errContains: "bind address already in use",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			err := processHTTPServerError(ctx, tt.err)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				// Verify error wrapping preserves original error
				if tt.err != nil {
					assert.ErrorIs(t, err, tt.err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
