package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/middleware"
	"github.com/openkcm/session-manager/internal/session"
)

// captureLoginCSRF returns a handler that records the value the generated
// binder would read: the "__Host-LoginCSRF" cookie.
func captureLoginCSRF(seen *string, found *bool) http.Handler {
	return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(session.LoginCSRFCookieName); err == nil {
			*seen = c.Value
			*found = true
		}
	})
}

func TestLoginCSRFCookieAliasMiddleware(t *testing.T) {
	t.Run("aliases configured cookie to canonical name", func(t *testing.T) {
		var seen string
		var found bool
		h := middleware.LoginCSRFCookieAliasMiddleware("LoginCSRF")(captureLoginCSRF(&seen, &found))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/sm/callback", nil)
		req.AddCookie(&http.Cookie{Name: "LoginCSRF", Value: "token-123"})
		h.ServeHTTP(httptest.NewRecorder(), req)

		require.True(t, found, "canonical cookie should be present after aliasing")
		assert.Equal(t, "token-123", seen)
	})

	t.Run("no-op when configured name is empty", func(t *testing.T) {
		var seen string
		var found bool
		h := middleware.LoginCSRFCookieAliasMiddleware("")(captureLoginCSRF(&seen, &found))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/sm/callback", nil)
		req.AddCookie(&http.Cookie{Name: "LoginCSRF", Value: "token-123"})
		h.ServeHTTP(httptest.NewRecorder(), req)

		assert.False(t, found, "should not alias when no configured name")
	})

	t.Run("no-op when configured name equals canonical", func(t *testing.T) {
		var seen string
		var found bool
		h := middleware.LoginCSRFCookieAliasMiddleware(session.LoginCSRFCookieName)(captureLoginCSRF(&seen, &found))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/sm/callback", nil)
		req.AddCookie(&http.Cookie{Name: session.LoginCSRFCookieName, Value: "token-123"})
		h.ServeHTTP(httptest.NewRecorder(), req)

		require.True(t, found)
		assert.Equal(t, "token-123", seen)
	})

	t.Run("does not overwrite existing canonical cookie", func(t *testing.T) {
		var seen string
		var found bool
		h := middleware.LoginCSRFCookieAliasMiddleware("LoginCSRF")(captureLoginCSRF(&seen, &found))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/sm/callback", nil)
		req.AddCookie(&http.Cookie{Name: session.LoginCSRFCookieName, Value: "canonical"})
		req.AddCookie(&http.Cookie{Name: "LoginCSRF", Value: "aliased"})
		h.ServeHTTP(httptest.NewRecorder(), req)

		require.True(t, found)
		assert.Equal(t, "canonical", seen, "existing canonical cookie must win")
	})
}
