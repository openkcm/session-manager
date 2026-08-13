package middleware

import (
	"net/http"

	"github.com/openkcm/session-manager/internal/session"
)

// LoginCSRFCookieAliasMiddleware maps a configured login-CSRF cookie name onto
// the canonical name that the generated OpenAPI handler reads
// (session.LoginCSRFCookieName, i.e. "__Host-LoginCSRF").
//
// The generated callback handler hard-codes r.Cookie("__Host-LoginCSRF"), and
// the "__Host-" prefix requires the cookie's Secure attribute — which browsers
// reject over plain http://. Local development therefore needs a non-"__Host-"
// cookie name, but the read side cannot be changed. This middleware bridges the
// gap: when configuredName differs from the canonical name and only the
// configured cookie is present, it appends a copy under the canonical name so
// the downstream binder finds it.
//
// It is a no-op when configuredName is empty or already equal to the canonical
// name, so production behavior is unchanged.
func LoginCSRFCookieAliasMiddleware(configuredName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if configuredName == "" || configuredName == session.LoginCSRFCookieName {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := r.Cookie(session.LoginCSRFCookieName); err != nil {
				if c, err := r.Cookie(configuredName); err == nil {
					r.AddCookie(&http.Cookie{Name: session.LoginCSRFCookieName, Value: c.Value})
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
