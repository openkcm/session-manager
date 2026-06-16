package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/grpc"
)

func TestStartGRPCServer_ContextCancellation(t *testing.T) {
	t.Run("gracefully shuts down when context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		cfg := &config.Config{
			GRPC: config.GRPCServer{
				GRPCServer: commoncfg.GRPCServer{
					Address: "localhost:0", // Use port 0 to get a random available port
				},
				ShutdownTimeout: 1 * time.Second,
			},
		}

		// Create minimal server instances
		oidcmappingsrv := grpc.NewOIDCMappingServer(nil)
		sessionsrv := grpc.NewSessionServer(ctx, nil, nil, 0, "")

		// Start the server in a goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- StartGRPCServer(ctx, cfg, oidcmappingsrv, sessionsrv)
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

func TestProcessGRPCServerError(t *testing.T) {
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
			name:        "non-nil error returns wrapped error",
			err:         errors.New("connection refused"),
			wantErr:     true,
			errContains: "gRPC server failed",
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
			err := processGRPCServerError(ctx, tt.err)

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
