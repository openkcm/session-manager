package credentials

import (
	"github.com/zitadel/oidc/pkg/client/rp"
	"github.com/zitadel/oidc/pkg/client/rs"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
)

// Provider constructs an rs.ResourceServer and an rp.RelyingParty for the given OIDC
// client and issuer. The issuer is required to discover the provider's introspection endpoint.
type Provider interface {
	ResourceServer(clientID, issuer string) (rs.ResourceServer, error)
	RelyingParty(oidc *oidcv1.OIDC, redirectURI string) (rp.RelyingParty, error)
}

type InsecureProvider struct{}

func (p InsecureProvider) ResourceServer(clientID, issuer string) (rs.ResourceServer, error) {
	return NewInsecureRS(clientID, issuer)
}

func (p InsecureProvider) RelyingParty(oidc *oidcv1.OIDC, redirectURI string) (rp.RelyingParty, error) {
	return NewInsecureRP(oidc.GetClientId(), oidc.GetIssuer(), redirectURI)
}
