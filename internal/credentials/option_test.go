package credentials

import (
	"testing"

	"github.com/zitadel/oidc/v3/pkg/oidc"
)

func TestWithDiscoveryConfig(t *testing.T) {
	disc := &oidc.DiscoveryConfiguration{
		Issuer:        "https://example.com",
		TokenEndpoint: "https://example.com/token",
	}
	opt := WithDiscoveryConfig(disc)
	if opt.discoveryConfig != disc {
		t.Errorf("discoveryConfig = %p, want %p", opt.discoveryConfig, disc)
	}
}
