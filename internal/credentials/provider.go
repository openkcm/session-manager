package credentials

import (
	"context"

	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/client/rs"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"

	"github.com/openkcm/session-manager/internal/debugtools"
)

// Provider constructs an rs.ResourceServer, an rp.RelyingParty, and fetches
// raw OIDC discovery configuration for the given issuer.
type Provider interface {
	ResourceServer(ctx context.Context, clientID, issuer string) (rs.ResourceServer, error)
	RelyingParty(ctx context.Context, oidc *oidcv1.OIDC, redirectURI string) (rp.RelyingParty, error)
	GetDiscoveryConfig(ctx context.Context, issuer string) (*oidc.DiscoveryConfiguration, error)
}

type InsecureProvider struct{}

func (p InsecureProvider) ResourceServer(ctx context.Context, clientID, issuer string) (rs.ResourceServer, error) {
	return NewInsecureRS(ctx, issuer, clientID)
}

func (p InsecureProvider) RelyingParty(ctx context.Context, oidc *oidcv1.OIDC, redirectURI string) (rp.RelyingParty, error) {
	return NewInsecureRP(ctx, oidc.GetIssuer(), oidc.GetClientId(), redirectURI)
}

// GetDiscoveryConfig performs a direct OIDC discovery.
func (p InsecureProvider) GetDiscoveryConfig(ctx context.Context, issuer string) (*oidc.DiscoveryConfiguration, error) {
	httpClient := *httphelper.DefaultHTTPClient
	httpClient.Transport = debugtools.DebugTransport(httpClient.Transport)
	return client.Discover(ctx, issuer, &httpClient)
}
