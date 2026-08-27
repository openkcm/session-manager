package session

import (
	"github.com/jellydator/ttlcache/v3"
	"github.com/zitadel/oidc/pkg/oidc"
)

type TokenResponse = tokenResponse

// WKOCCache returns the internal wkocCache for testing purposes.
func (m *Manager) WKOCCache() *ttlcache.Cache[string, *oidc.DiscoveryConfiguration] {
	return m.wkocCache
}
