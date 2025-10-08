package oidc

import "context"

// ProviderRepository allows to read OIDC provider data for a tenant stored in the context.
type ProviderRepository interface {
	GetForTenant(ctx context.Context, tenantID string) (Provider, error)
	Get(ctx context.Context, issuerURL string) (Provider, error)
	Create(ctx context.Context, tenantID string, provider Provider) error
	Delete(ctx context.Context, tenantID string, provider Provider) error
	Update(ctx context.Context, tenantID string, provider Provider) error
}
