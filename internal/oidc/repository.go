package oidc

import "context"

// ProviderRepository allows to read OIDC provider data for a tenant stored in the context.
type ProviderRepository interface {
	Get(ctx context.Context, tenantID string) (Provider, error)
	Create(ctx context.Context, tenantID string, provider Provider) error
	Delete(ctx context.Context, tenantID string) error
	Update(ctx context.Context, tenantID string, provider Provider) error
}
