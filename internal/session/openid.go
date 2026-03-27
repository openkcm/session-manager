package session

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/openkcm/session-manager/internal/trust"
)

// getOpenIDConfig fetches and caches the OIDC discovery configuration for the given issuer.
func (m *Manager) getOpenIDConfig(ctx context.Context, issuerURL string, mapping trust.OIDCMapping) (*oidc.DiscoveryConfiguration, error) {
	const wkocPrefix = "wkoc_"

	issuer, err := url.Parse(issuerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing issuer url: %w", err)
	}
	if issuer.Scheme == "http" && !m.allowHttpScheme {
		return nil, errors.New("insecure http issuer url is not allowed")
	}

	hashedSuffix := sha256.Sum256([]byte(issuerURL))
	cacheKey := wkocPrefix + base64.RawURLEncoding.EncodeToString(hashedSuffix[:])

	cached, ok := m.cache.Get(cacheKey)
	if ok {
		value, ok := cached.(*oidc.DiscoveryConfiguration)
		if ok {
			return value, nil
		}
		m.cache.Delete(cacheKey)
	}

	httpClient := m.httpClient(mapping)
	httpClient.Timeout = 30 * time.Second

	cfg, err := client.Discover(ctx, issuerURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("discovering openid configuration: %w", err)
	}
	m.cache.Set(cacheKey, cfg, 0)

	return cfg, nil
}
