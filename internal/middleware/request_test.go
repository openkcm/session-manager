package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openkcm/session-manager/internal/middleware"
)

func TestRequestMiddleware(t *testing.T) {
	var calledNextHandler bool

	rec := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNextHandler = true

		insertedRequest, err := middleware.RequestFromContext(r.Context())
		if err != nil {
			t.Fatalf("RequestFromContext should not return an error: %v", err)
		}

		if req != insertedRequest {
			t.Fatal("Insected Request must be the same instance")
		}
	})

	handler := middleware.RequestMiddleware(next)
	handler.ServeHTTP(rec, req)

	if !calledNextHandler {
		t.Fatal("The next handler was not called")
	}
}
