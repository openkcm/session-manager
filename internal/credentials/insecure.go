package credentials

import (
	"github.com/zitadel/oidc/pkg/client/rp"
	"github.com/zitadel/oidc/pkg/client/rs"

	httphelper "github.com/zitadel/oidc/pkg/http"
)

// NewInsecureRS returns an rs.ResourceServer that only sends the client_id in the
// without any client authentication. It must not be used in production.
func NewInsecureRS(clientID, issuer string) (rs.ResourceServer, error) {
	httpClient := *httphelper.DefaultHTTPClient // Create a shallow copy
	httpClient.Transport = debugTransport(httpClient.Transport)
	return newResourceServer(issuer, &httpClient, clientSecretPostAuth(clientID, ""))
}

// NewInsecureRP returns an rp.RelyingParty that sends only the client_id
// without any client authentication. It must not be used in production.
func NewInsecureRP(clientID, issuer, redirectURI string) (rp.RelyingParty, error) {
	httpClient := *httphelper.DefaultHTTPClient // Create a shallow copy
	httpClient.Transport = debugTransport(httpClient.Transport)
	return rp.NewRelyingPartyOIDC(issuer, clientID, "", redirectURI, nil)
}
