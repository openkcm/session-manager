package credentials

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// buildRPHTTPClient returns an HTTP client for use with rp.NewRelyingPartyOIDC.
// When a WithDiscoveryConfig option is present, the returned client's transport
// intercepts the well-known discovery URL and serves the cached document from
// memory, so rp.NewRelyingPartyOIDC performs no network round-trip for discovery.
// All other requests are forwarded to base.
func buildRPHTTPClient(issuer string, base http.RoundTripper, opts []Option) (*http.Client, error) {
	for _, opt := range opts {
		if opt.discoveryConfig != nil {
			discJSON, err := json.Marshal(opt.discoveryConfig)
			if err != nil {
				return nil, err
			}
			return &http.Client{
				Transport: &cachedDiscoveryTransport{
					base:         base,
					wellKnownURL: strings.TrimSuffix(issuer, "/") + oidc.DiscoveryEndpoint,
					body:         discJSON,
				},
			}, nil
		}
	}
	return &http.Client{Transport: base}, nil
}

// cachedDiscoveryTransport serves a pre-marshaled discovery document for the
// well-known URL and forwards all other requests to the base RoundTripper.
type cachedDiscoveryTransport struct {
	base         http.RoundTripper
	wellKnownURL string
	body         []byte
}

func (t *cachedDiscoveryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() == t.wellKnownURL {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(t.body)),
			Request:    req,
		}, nil
	}
	return t.base.RoundTrip(req)
}
