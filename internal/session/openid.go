package session

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

func (m *Manager) getOpenIDConfig(ctx context.Context, issuerURL string) (*oidc.DiscoveryConfiguration, error) {
	const wkocPrefix = "wkoc_"

	issuer, err := url.Parse(issuerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing issuer url: %w", err)
	}
	if issuer.Scheme == "http" && !m.allowHttpScheme {
		return nil, fmt.Errorf("insecure http issuer url is not allowed")
	}

	// first check the cache for a recent WKOC configuration for this issuer
	hashedSuffix := sha256.Sum256([]byte(issuerURL))
	cacheKey := wkocPrefix + base64.RawURLEncoding.EncodeToString(hashedSuffix[:])

	cache, ok := m.cache.Get(cacheKey)
	if ok {
		value, ok := cache.(*oidc.DiscoveryConfiguration)
		if ok {
			return value, nil
		}
		m.cache.Delete(cacheKey)
	}

	// otherwise, fetch the configuration and cache it
	httpClient := m.secureClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	cfg, err := client.Discover(ctx, issuerURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("discovering openid configuration: %w", err)
	}
	m.cache.Set(cacheKey, cfg, 0)

	return cfg, nil
}
