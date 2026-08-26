package session

import (
	"github.com/jellydator/ttlcache/v3"
	"github.com/openkcm/common-sdk/pkg/oidc"
)

type TokenResponse = tokenResponse

// WKOCCache returns the internal wkocCache for testing purposes.
func (m *Manager) WKOCCache() *ttlcache.Cache[string, *oidc.Configuration] {
	return m.wkocCache
}
