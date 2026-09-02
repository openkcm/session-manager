package credentials

import (
	"context"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/client/rs"

	httphelper "github.com/zitadel/oidc/v3/pkg/http"

	"github.com/openkcm/session-manager/internal/debugtools"
)

// NewInsecureRS returns an rs.ResourceServer that only sends the client_id in the request body
// without any client authentication. It must not be used in production.
func NewInsecureRS(ctx context.Context, issuer, clientID string) (rs.ResourceServer, error) {
	httpClient := *httphelper.DefaultHTTPClient // Create a shallow copy
	httpClient.Transport = debugtools.DebugTransport(httpClient.Transport)
	return newResourceServer(ctx, issuer, &httpClient, clientSecretPostAuth(clientID, ""))
}

// NewInsecureRP returns an rp.RelyingParty that sends only the client_id
// without any client authentication. It must not be used in production.
func NewInsecureRP(ctx context.Context, clientID, issuer, redirectURI string) (rp.RelyingParty, error) {
	httpClient := *httphelper.DefaultHTTPClient // Create a shallow copy
	httpClient.Transport = debugtools.DebugTransport(httpClient.Transport)
	return rp.NewRelyingPartyOIDC(ctx, issuer, clientID, "", redirectURI, nil)
}
