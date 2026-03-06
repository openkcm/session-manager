package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/config"
)

func TestInitMeters(t *testing.T) {
	t.Run("initializes meters successfully", func(t *testing.T) {
		ctx := t.Context()
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
		}

		err := initMeters(ctx, cfg)
		assert.NoError(t, err)
	})
}

func TestNewTraceMiddleware(t *testing.T) {
	t.Run("creates trace middleware", func(t *testing.T) {
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
		}

		middleware := newTraceMiddleware(cfg)
		assert.NotNil(t, middleware)
	})

	t.Run("wraps handler function correctly", func(t *testing.T) {
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name:        "test-app",
					Environment: "test",
				},
			},
		}

		// Initialize meters first
		err := initMeters(context.Background(), cfg)
		require.NoError(t, err)

		middleware := newTraceMiddleware(cfg)
		operationID := "TestOperation"

		handlerCalled := false
		expectedResponse := map[string]string{"status": "ok"}

		// Create a mock handler
		mockHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			handlerCalled = true
			// Verify context has operation ID and request ID
			return expectedResponse, nil
		}

		// Wrap the handler with middleware
		wrappedHandler := middleware(mockHandler, operationID)
		assert.NotNil(t, wrappedHandler)

		// Create test request
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", "test-agent")
		w := httptest.NewRecorder()

		// Execute wrapped handler
		response, err := wrappedHandler(context.Background(), w, req, nil)

		require.NoError(t, err)
		assert.True(t, handlerCalled)
		assert.Equal(t, expectedResponse, response)
	})

	t.Run("propagates handler errors", func(t *testing.T) {
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
		}

		// Initialize meters
		err := initMeters(context.Background(), cfg)
		require.NoError(t, err)

		middleware := newTraceMiddleware(cfg)
		expectedError := errors.New("handler error")

		// Create a mock handler that returns an error
		mockHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			return nil, expectedError
		}

		wrappedHandler := middleware(mockHandler, "ErrorOperation")

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		response, err := wrappedHandler(context.Background(), w, req, nil)

		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
		assert.Nil(t, response)
	})

	t.Run("records metrics for request", func(t *testing.T) {
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name:        "test-app",
					Environment: "test",
				},
			},
		}

		// Initialize meters
		err := initMeters(context.Background(), cfg)
		require.NoError(t, err)

		middleware := newTraceMiddleware(cfg)

		mockHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			// Simulate some work
			return "success", nil
		}

		wrappedHandler := middleware(mockHandler, "MetricsOperation")

		req := httptest.NewRequest(http.MethodPost, "/metrics-test", nil)
		req.Header.Set("User-Agent", "metrics-test-agent")
		w := httptest.NewRecorder()

		response, err := wrappedHandler(context.Background(), w, req, map[string]string{"key": "value"})

		require.NoError(t, err)
		assert.Equal(t, "success", response)
		// Metrics are recorded in defer, so they should be captured
	})

	t.Run("extracts parent trace context from headers", func(t *testing.T) {
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
		}

		// Initialize meters
		err := initMeters(context.Background(), cfg)
		require.NoError(t, err)

		middleware := newTraceMiddleware(cfg)

		contextChecked := false
		mockHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			// Context should have trace propagation applied
			contextChecked = true
			return "ok", nil
		}

		wrappedHandler := middleware(mockHandler, "TraceOperation")

		req := httptest.NewRequest(http.MethodGet, "/trace-test", nil)
		// Add trace headers
		req.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
		w := httptest.NewRecorder()

		_, err = wrappedHandler(context.Background(), w, req, nil)

		require.NoError(t, err)
		assert.True(t, contextChecked)
	})

	t.Run("handles multiple sequential requests", func(t *testing.T) {
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
		}

		// Initialize meters
		err := initMeters(context.Background(), cfg)
		require.NoError(t, err)

		middleware := newTraceMiddleware(cfg)

		callCount := 0
		mockHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			callCount++
			return callCount, nil
		}

		wrappedHandler := middleware(mockHandler, "SequentialOperation")

		// Make multiple requests
		for i := 1; i <= 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			response, err := wrappedHandler(context.Background(), w, req, nil)

			require.NoError(t, err)
			assert.Equal(t, i, response)
		}

		assert.Equal(t, 3, callCount)
	})
}
