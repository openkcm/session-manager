package credentials

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/zitadel/oidc/pkg/client"
	"github.com/zitadel/oidc/pkg/client/rs"

	httphelper "github.com/zitadel/oidc/pkg/http"
)

// resourceServer is a rs.ResourceServer implementation.
type resourceServer struct {
	httpClient       *http.Client
	introspectionURL string
	authFn           any
}

var _ rs.ResourceServer = (*resourceServer)(nil)

// newResourceServer discovers the issuer's OIDC configuration using httpClient
// and returns a resource server that authorizes introspection requests with
// authFn (a zitadel httphelper.FormAuthorization or RequestAuthorization).
func newResourceServer(issuer string, httpClient *http.Client, authFn any) (*resourceServer, error) {
	conf, err := client.Discover(issuer, httpClient)
	if err != nil {
		return nil, fmt.Errorf("discovering OIDC configuration for issuer %q: %w", issuer, err)
	}

	return &resourceServer{
		httpClient:       httpClient,
		introspectionURL: conf.IntrospectionEndpoint,
		authFn:           authFn,
	}, nil
}

// HttpClient implements [rs.ResourceServer].
func (r *resourceServer) HttpClient() *http.Client {
	return r.httpClient
}

// IntrospectionURL implements [rs.ResourceServer].
func (r *resourceServer) IntrospectionURL() string {
	return r.introspectionURL
}

// AuthFn implements [rs.ResourceServer].
func (r *resourceServer) AuthFn() (any, error) {
	return r.authFn, nil
}

// clientSecretPostAuth returns a form authorization following the
// 'client_secret_post' method: the client_id and, when set, the client_secret
// are added to the request body.
// https://openid.net/specs/openid-connect-core-1_0.html#ClientAuthentication
func clientSecretPostAuth(clientID, clientSecret string) httphelper.FormAuthorization {
	return func(form url.Values) {
		form.Set("client_id", clientID)
		if clientSecret != "" {
			form.Set("client_secret", clientSecret)
		}
	}
}
