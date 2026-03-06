package server

import (
	"context"
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
