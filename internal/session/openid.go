package session

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/oidc"
)

func (m *Manager) getOpenIDConfig(ctx context.Context, issuerURL string) (*oidc.Configuration, error) {
	const wkocPrefix = "wkoc_"

	// first check the cache for a recent WKOC configuration for this issuer
	cacheKey := wkocPrefix + issuerURL
	cache, ok := m.cache.Get(cacheKey)
	if ok {
		//nolint:forcetypeassert
		return cache.(*oidc.Configuration), nil
	}

	// otherwise, fetch the configuration and cache it
	provider, err := oidc.NewProvider(issuerURL, []string{}, oidc.WithAllowHttpScheme(m.allowHttpScheme))
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
