package trust

import "context"

// OIDCMappingRepository allows to read OIDC provider data for a tenant stored in the context.
type OIDCMappingRepository interface {
	Get(ctx context.Context, tenantID string) (OIDCMapping, error)
	Create(ctx context.Context, tenantID string, provider OIDCMapping) error
	Delete(ctx context.Context, tenantID string) error
	Update(ctx context.Context, tenantID string, provider OIDCMapping) error
}
