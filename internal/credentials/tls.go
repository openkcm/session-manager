package credentials

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/client/rs"

	"github.com/openkcm/session-manager/internal/debugtools"
)

// NewTLSRS returns an rs.ResourceServer that authenticates the client using
// mutual TLS: the TLS client certificate configured in tlsConfig proves the
// client's identity, while the client_id is still sent in the request body.
func NewTLSRS(ctx context.Context, issuer, clientID string, tlsConfig *tls.Config, opts ...Option) (rs.ResourceServer, error) {
	httpClient := &http.Client{
		Transport: debugtools.DebugTransport(&http.Transport{
			TLSClientConfig: tlsConfig,
		}),
	}
	return newResourceServer(ctx, issuer, httpClient, clientSecretPostAuth(clientID, ""), opts...)
}

// NewTLSRP returns an rp.RelyingParty that authenticates the client using
// mutual TLS: the TLS client certificate configured in tlsConfig proves the
// client's identity, while the client_id is still sent in the request body.
func NewTLSRP(ctx context.Context, issuer, clientID, redirectURI string, tlsConfig *tls.Config, opts ...Option) (rp.RelyingParty, error) {
	base := debugtools.DebugTransport(&http.Transport{TLSClientConfig: tlsConfig})
	httpClient, err := buildRPHTTPClient(issuer, base, opts)
	if err != nil {
		return nil, err
	}
	return rp.NewRelyingPartyOIDC(ctx, issuer, clientID, "", redirectURI, nil, rp.WithHTTPClient(httpClient))
}
