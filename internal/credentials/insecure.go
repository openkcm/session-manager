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
func NewInsecureRS(ctx context.Context, issuer, clientID string, opts ...Option) (rs.ResourceServer, error) {
	httpClient := *httphelper.DefaultHTTPClient
	httpClient.Transport = debugtools.DebugTransport(httpClient.Transport)
	return newResourceServer(ctx, issuer, &httpClient, clientSecretPostAuth(clientID, ""), opts...)
}

// NewInsecureRP returns an rp.RelyingParty that sends only the client_id
// without any client authentication. It must not be used in production.
func NewInsecureRP(ctx context.Context, clientID, issuer, redirectURI string, opts ...Option) (rp.RelyingParty, error) {
	base := debugtools.DebugTransport(httphelper.DefaultHTTPClient.Transport)
	httpClient, err := buildRPHTTPClient(issuer, base, opts)
	if err != nil {
		return nil, err
	}
	return rp.NewRelyingPartyOIDC(ctx, issuer, clientID, "", redirectURI, nil, rp.WithHTTPClient(httpClient))
}
