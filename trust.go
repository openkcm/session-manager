package sessionmanager

import (
	"context"

	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"
)

type Trust interface {
	// ApplyMapping applies and stores the provided Trust.
	ApplyMapping(ctx context.Context, trust *trustv1.Trust) error
	// BlockMapping sets the Blocked flag to true for the OIDC mapping associated with the given tenantID.
	// If the mapping is already blocked, it does nothing.
	// Returns an error if the mapping cannot be retrieved or updated.
	BlockMapping(ctx context.Context, tenantID string) error
	// RemoveMapping removes the specified mapping from the trust.
	RemoveMapping(ctx context.Context, tenantID string) error
	// UnblockMapping sets the Blocked flag to false for the OIDC mapping associated with the given tenantID.
	// If the mapping is not blocked, it does nothing.
	// Returns an error if the mapping cannot be retrieved or updated.
	UnblockMapping(ctx context.Context, tenantID string) error
	// Get returns a trust message with optional extensions set.
	Get(ctx context.Context, tenantID string) (*trustv1.Trust, error)
}
