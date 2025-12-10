package middleware

import (
	"context"
	"errors"
	"net/http"
)

// Using an unexported type prevents key collisions from other packages.
type requestKey struct{}

// RequestMiddleware is an http.Handler middleware that injects
// the response writer for the original *http.Request into the context.
func RequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), requestKey{}, r)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestFromContext is a helper function that retrieves the response
// writer from the context.
func RequestFromContext(ctx context.Context) (*http.Request, error) {
	dom, ok := ctx.Value(requestKey{}).(*http.Request)
	if !ok {
		return nil, errors.New("response writer not found in context")
	}

	return dom, nil
}
