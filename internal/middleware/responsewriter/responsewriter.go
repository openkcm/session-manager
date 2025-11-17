// Package responsewriter provides utilities to inject the response writer
// for the original *http.Request into the context and also retrieve it.
package responsewriter

import (
	"context"
	"errors"
	"net/http"
)

// Using an unexported type prevents key collisions from other packages.
type responseWriterKey string

// ResponseWriterKey is the context key for the response writer.
const ResponseWriterKey responseWriterKey = "response-writer"

// ResponseWriterMiddleware is an http.Handler middleware that injects
// the response writer for the original *http.Request into the context.
func ResponseWriterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ResponseWriterKey, w)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ResponseWriterFromContext is a helper function that retrieves the response
// writer from the context.
func ResponseWriterFromContext(ctx context.Context) (http.ResponseWriter, error) {
	dom, ok := ctx.Value(ResponseWriterKey).(http.ResponseWriter)
	if !ok {
		return nil, errors.New("response writer not found in context")
	}
	return dom, nil
}
