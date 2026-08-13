package oidctrust

import (
	"context"

	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"
)

// TrustRepository allows to read OIDC trust data for a tenant stored in the context.
type TrustRepository interface {
	Get(ctx context.Context, tenantID string) (*trustv1.Trust, error)
	Create(ctx context.Context, trust *trustv1.Trust) error
	Delete(ctx context.Context, tenantID string) error
	Update(ctx context.Context, trust *trustv1.Trust) error
}
