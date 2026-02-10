package session

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/openid"
)

func (m *Manager) getOpenIDConfig(ctx context.Context, issuerURL string) (*openid.Configuration, error) {
	const wkocPrefix = "wkoc_"

	// first check the cache for a recent WKOC configuration for this issuer
	cacheKey := wkocPrefix + issuerURL
	cache, ok := m.cache.Get(cacheKey)
	if ok {
		//nolint:forcetypeassert
		return cache.(*openid.Configuration), nil
	}

	// otherwise, fetch the configuration and cache it
	provider, err := openid.NewProvider(issuerURL, []string{})
	if err != nil {
		return nil, err
	}
	cfg, err := provider.GetConfiguration(ctx)
	if err != nil {
		return nil, err
	}
	m.cache.Set(cacheKey, cfg, 0)

	return cfg, nil
}
