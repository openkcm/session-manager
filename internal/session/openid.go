package session

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"

	"github.com/jellydator/ttlcache/v3"
	"github.com/zitadel/oidc/pkg/client"
	"github.com/zitadel/oidc/pkg/oidc"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/validation"
)

func (m *Manager) getOpenIDConfig(ctx context.Context, issuerURL string) (*oidc.DiscoveryConfiguration, error) {
	// first check the cache for a recent WKOC configuration for this issuer
	hashedSuffix := sha256.Sum256([]byte(issuerURL))
	cacheKey := base64.RawURLEncoding.EncodeToString(hashedSuffix[:])
	if item := m.wkocCache.Get(cacheKey); item != nil {
		return item.Value(), nil
	}

	if err := validation.SecureScheme(issuerURL, m.allowHttpScheme); err != nil {
		return nil, err
	}

	config, err := client.Discover(issuerURL, http.DefaultClient)
	if err != nil {
		slogctx.Error(ctx, "Could not get OIDC provider configuration",
			"issuerURL", issuerURL, "error", err)
		return nil, err
	}

	// Cache the result with TTL
	m.wkocCache.Set(cacheKey, config, ttlcache.DefaultTTL)

	return config, nil
}
