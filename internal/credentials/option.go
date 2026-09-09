package credentials

import "github.com/zitadel/oidc/v3/pkg/oidc"

// Option configures a credentials constructor.
type Option struct {
	discoveryConfig *oidc.DiscoveryConfiguration
}

// WithDiscoveryConfig provides a pre-fetched OIDC discovery configuration.
// When set, constructors use the provided config directly and skip the
// OIDC discovery HTTP call.
func WithDiscoveryConfig(disc *oidc.DiscoveryConfiguration) Option {
	return Option{discoveryConfig: disc}
}
