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
func NewTLSRS(ctx context.Context, issuer, clientID string, tlsConfig *tls.Config) (rs.ResourceServer, error) {
	httpClient := &http.Client{
		Transport: debugtools.DebugTransport(&http.Transport{
			TLSClientConfig: tlsConfig,
		}),
	}

	return newResourceServer(ctx, issuer, httpClient, clientSecretPostAuth(clientID, ""))
}

// NewTLSRP returns an rp.RelyingParty that authenticates the client using
// mutual TLS: the TLS client certificate configured in tlsConfig proves the
// client's identity, while the client_id is still sent in the request body.
func NewTLSRP(ctx context.Context, clientID, issuer, redirectURI string, tlsConfig *tls.Config) (rp.RelyingParty, error) {
	httpClient := &http.Client{
		Transport: debugtools.DebugTransport(&http.Transport{
			TLSClientConfig: tlsConfig,
		}),
	}

	return rp.NewRelyingPartyOIDC(ctx, issuer, clientID, "", redirectURI, nil, rp.WithHTTPClient(httpClient))
}
