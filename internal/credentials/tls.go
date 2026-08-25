package credentials

import (
	"crypto/tls"
	"net/http"

	"github.com/zitadel/oidc/pkg/client/rp"
	"github.com/zitadel/oidc/pkg/client/rs"
)

// NewTLSRS returns an rs.ResourceServer that authenticates the client using
// mutual TLS: the TLS client certificate configured in tlsConfig proves the
// client's identity, while the client_id is still sent in the request body.
func NewTLSRS(clientID, issuer string, tlsConfig *tls.Config) (rs.ResourceServer, error) {
	httpClient := &http.Client{
		Transport: debugTransport(&http.Transport{
			TLSClientConfig: tlsConfig,
		}),
	}

	return newResourceServer(issuer, httpClient, clientSecretPostAuth(clientID, ""))
}

// NewTLSRP returns an rp.RelyingParty that authenticates the client using
// mutual TLS: the TLS client certificate configured in tlsConfig proves the
// client's identity, while the client_id is still sent in the request body.
func NewTLSRP(clientID, issuer, redirectURI string, tlsConfig *tls.Config) (rp.RelyingParty, error) {
	httpClient := &http.Client{
		Transport: debugTransport(&http.Transport{
			TLSClientConfig: tlsConfig,
		}),
	}

	return rp.NewRelyingPartyOIDC(issuer, clientID, "", redirectURI, nil, rp.WithHTTPClient(httpClient))
}
