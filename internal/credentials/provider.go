package credentials

import (
	"context"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/client/rs"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
)

// Provider constructs an rs.ResourceServer and an rp.RelyingParty for the given OIDC
// client and issuer. The issuer is required to discover the provider's introspection endpoint.
type Provider interface {
	ResourceServer(ctx context.Context, clientID, issuer string) (rs.ResourceServer, error)
	RelyingParty(ctx context.Context, oidc *oidcv1.OIDC, redirectURI string) (rp.RelyingParty, error)
}

type InsecureProvider struct{}

func (p InsecureProvider) ResourceServer(ctx context.Context, clientID, issuer string) (rs.ResourceServer, error) {
	return NewInsecureRS(ctx, issuer, clientID)
}

func (p InsecureProvider) RelyingParty(ctx context.Context, oidc *oidcv1.OIDC, redirectURI string) (rp.RelyingParty, error) {
	return NewInsecureRP(ctx, oidc.GetClientId(), oidc.GetIssuer(), redirectURI)
}
