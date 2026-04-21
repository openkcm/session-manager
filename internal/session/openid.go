package session

import (
	"context"
	"crypto/sha256"
	"encoding/base64"

	"github.com/jellydator/ttlcache/v3"
	"github.com/openkcm/common-sdk/pkg/oidc"
)

func (m *Manager) getOpenIDConfig(ctx context.Context, issuerURL string) (*oidc.Configuration, error) {
	// first check the cache for a recent WKOC configuration for this issuer
	hashedSuffix := sha256.Sum256([]byte(issuerURL))
	cacheKey := base64.RawURLEncoding.EncodeToString(hashedSuffix[:])
	if item := m.wkocCache.Get(cacheKey); item != nil {
		return item.Value(), nil
	}

	// otherwise, fetch the configuration
	provider, err := oidc.NewProvider(issuerURL, []string{},
		oidc.WithAllowHttpScheme(m.allowHttpScheme),
	)
	if err != nil {
		return nil, err
	}
	cfg, err := provider.GetConfiguration(ctx)
	if err != nil {
		return nil, err
	}

	// Cache the result with TTL
	m.wkocCache.Set(cacheKey, cfg, ttlcache.DefaultTTL)

	return cfg, nil
}
