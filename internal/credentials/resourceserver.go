package credentials

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rs"

	httphelper "github.com/zitadel/oidc/v3/pkg/http"
)

// resourceServer is a rs.ResourceServer implementation.
type resourceServer struct {
	httpClient       *http.Client
	tokenEndpoint    string
	introspectionURL string
	authFn           httphelper.FormAuthorization
}

var _ rs.ResourceServer = (*resourceServer)(nil)

// newResourceServer returns a resource server for issuer. When a WithDiscoveryConfig
// option is provided the OIDC discovery HTTP call is skipped and the endpoints are
// taken from the supplied config; otherwise discovery is performed via httpClient.
func newResourceServer(ctx context.Context, issuer string, httpClient *http.Client, authFn httphelper.FormAuthorization, opts ...Option) (*resourceServer, error) {
	var tokenEndpoint, introspectionURL string
	for _, opt := range opts {
		if opt.discoveryConfig != nil {
			tokenEndpoint = opt.discoveryConfig.TokenEndpoint
			introspectionURL = opt.discoveryConfig.IntrospectionEndpoint
			break
		}
	}

	if tokenEndpoint == "" || introspectionURL == "" {
		conf, err := client.Discover(ctx, issuer, httpClient)
		if err != nil {
			return nil, fmt.Errorf("discovering OIDC configuration for issuer %q: %w", issuer, err)
		}
		tokenEndpoint = conf.TokenEndpoint
		introspectionURL = conf.IntrospectionEndpoint
	}

	return &resourceServer{
		httpClient:       httpClient,
		tokenEndpoint:    tokenEndpoint,
		introspectionURL: introspectionURL,
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

// TokenEndpoint implements [rs.ResourceServer].
func (r *resourceServer) TokenEndpoint() string {
	return r.tokenEndpoint
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
