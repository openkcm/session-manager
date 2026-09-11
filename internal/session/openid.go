package session

import (
	"context"

	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/openkcm/session-manager/internal/validation"
)

func (m *Manager) getOpenIDConfig(ctx context.Context, issuerURL string) (*oidc.DiscoveryConfiguration, error) {
	if err := validation.SecureScheme(issuerURL, m.allowHttpScheme); err != nil {
		return nil, err
	}
	return m.cProvider.GetDiscoveryConfig(ctx, issuerURL)
}
