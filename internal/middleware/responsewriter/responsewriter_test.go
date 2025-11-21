package responsewriter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/middleware/responsewriter"
)

func TestResponseWriterMiddleware(t *testing.T) {
	// A flag to ensure the next handler was executed
	var calledNextHandler bool

	// A fake ResponseWriter (recorder) to pass into the middleware
	rec := httptest.NewRecorder()

	// A standard request object
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// 1. Define the next handler that verifies context injection
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNextHandler = true

		// a) Verify the injected writer matches the original writer
		injectedWriter, err := responsewriter.ResponseWriterFromContext(r.Context())
		//nolint:testifylint
		require.NoError(t, err, "ResponseWriterFromContext should not return an error")
		assert.Same(t, rec, injectedWriter, "Injected ResponseWriter must be the same instance")

		// b) Verify the ResponseWriter is the instance passed to ServeHTTP
		assert.Same(t, rec, w, "ResponseWriter passed to next handler must be the original instance")
	})

	// 2. Wrap and execute the middleware
	handler := responsewriter.ResponseWriterMiddleware(next)
	handler.ServeHTTP(rec, req)

	// 3. Final assertion: check that the next handler was called
	assert.True(t, calledNextHandler, "The next handler was not executed")
}

func TestResponseWriterFromContext(t *testing.T) {
	// Use ResponseRecorder as a concrete implementation of http.ResponseWriter
	rec := httptest.NewRecorder()

	t.Run("Success", func(t *testing.T) {
		// 1. Setup: Inject the ResponseWriter into the context
		ctx := context.WithValue(context.Background(), responsewriter.ResponseWriterKey, rec)

		// 2. Execution
		retrievedWriter, err := responsewriter.ResponseWriterFromContext(ctx)

		// 3. Assertions
		require.NoError(t, err, "Should successfully retrieve the ResponseWriter")
		assert.Same(t, rec, retrievedWriter, "Retrieved writer must be the same instance")
	})

	t.Run("Failure_KeyNotFound", func(t *testing.T) {
		// 1. Setup: Use an empty context
		ctx := context.Background()

		// 2. Execution
		_, err := responsewriter.ResponseWriterFromContext(ctx)

		// 3. Assertions
		require.Error(t, err, "Should return an error when the key is not found")
		assert.Contains(t, err.Error(), "not found in context", "Error message should indicate key absence")
	})

	t.Run("Failure_WrongType", func(t *testing.T) {
		// 1. Setup: Inject a string instead of a ResponseWriter
		ctx := context.WithValue(context.Background(), responsewriter.ResponseWriterKey, "i-am-a-string")

		// 2. Execution
		_, err := responsewriter.ResponseWriterFromContext(ctx)

		// 3. Assertions
		require.Error(t, err, "Should return an error when the value type is incorrect")
		assert.Contains(t, err.Error(), "not found in context", "Error should indicate type assertion failure (which is handled as not found)")
	})
}
