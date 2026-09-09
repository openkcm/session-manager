package credentials

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/zitadel/oidc/v3/pkg/client/rs"
	zitadeloidc "github.com/zitadel/oidc/v3/pkg/oidc"

	httphelper "github.com/zitadel/oidc/v3/pkg/http"
)

// newDiscoveryServer starts an OIDC discovery endpoint that advertises the
// given introspection endpoint. The returned server's URL is a valid issuer:
// the discovery document reports it as its own issuer, which the zitadel client
// validates.
func newDiscoveryServer(t *testing.T, introspectionEndpoint string) *httptest.Server {
	t.Helper()

	var issuer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuer,
			"introspection_endpoint": introspectionEndpoint,
			"authorization_endpoint": issuer + "/authorize",
			"token_endpoint":         issuer + "/token",
			"jwks_uri":               issuer + "/keys",
		})
	}))
	issuer = srv.URL
	t.Cleanup(srv.Close)

	return srv
}

// authForm resolves the resource server's AuthFn and applies it to an empty
// form, returning the resulting values. It fails the test if AuthFn is not a
// form authorization.
func authForm(t *testing.T, r rs.ResourceServer) url.Values {
	t.Helper()

	authFn, err := r.AuthFn()
	if err != nil {
		t.Fatalf("AuthFn() error = %v", err)
	}
	fn, ok := authFn.(httphelper.FormAuthorization)
	if !ok {
		t.Fatalf("AuthFn() = %T, want httphelper.FormAuthorization", authFn)
	}

	form := url.Values{}
	fn(form)
	return form
}

func TestNewResourceServer_WithDiscoveryConfig(t *testing.T) {
	const tokenEndpoint = "https://example.com/token"
	const introspectionURL = "https://example.com/introspect"

	disc := &zitadeloidc.DiscoveryConfiguration{
		Issuer:                "https://example.com",
		TokenEndpoint:         tokenEndpoint,
		IntrospectionEndpoint: introspectionURL,
	}
	authFn := clientSecretPostAuth("client-id", "")

	srv, err := newResourceServer(t.Context(), "https://example.com", http.DefaultClient, authFn, WithDiscoveryConfig(disc))
	if err != nil {
		t.Fatalf("newResourceServer() error = %v", err)
	}
	if srv.IntrospectionURL() != introspectionURL {
		t.Errorf("IntrospectionURL() = %q, want %q", srv.IntrospectionURL(), introspectionURL)
	}
	if srv.TokenEndpoint() != tokenEndpoint {
		t.Errorf("TokenEndpoint() = %q, want %q", srv.TokenEndpoint(), tokenEndpoint)
	}
}
