package sessionmanager

import (
	"context"

	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"
)

type Trust interface {
	ApplyMapping(ctx context.Context, trust *trustv1.Trust) error
	BlockMapping(ctx context.Context, tenantID string) error
	RemoveMapping(ctx context.Context, tenantID string) error
	UnblockMapping(ctx context.Context, tenantID string) error
	Get(ctx context.Context, tenantID string) (*trustv1.Trust, error)
}
