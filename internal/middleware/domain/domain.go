// Package domain provides utilities to inject and retrieve the original request
// domain in and from the context.
package domain

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// Using an unexported type prevents key collisions from other packages.
type contextKey string

// DomainKey is the context key used to store the domain of the original request.
const DomainKey contextKey = "domain"

// DomainMiddleware is an http.Handler middleware that injects the domain
// of the original *http.Request into the context for later handlers to access.
func DomainMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dom := domainFromRequest(r.URL)
		ctx := context.WithValue(r.Context(), DomainKey, dom)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// DomainFromContext is a helper function that retrieves the domain
// from the context.
func DomainFromContext(ctx context.Context) (string, error) {
	dom, ok := ctx.Value(DomainKey).(string)
	if !ok {
		return "", errors.New("domain not found in context")
	}
	return dom, nil
}

// domainFromRequest constructs the domain string from the given URL
// by combining its scheme and host and omitting any other parts like
// path, query parameters, or fragments.
func domainFromRequest(requrl *url.URL) string {
	return requrl.Host
}
