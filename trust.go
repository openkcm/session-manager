package sessionmanager

import (
	"context"

	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"
)

type Trust interface {
	// Apply applies and stores the provided Trust.
	Apply(ctx context.Context, trust *trustv1.Trust) error
	// Block sets the Blocked flag to true for the trust associated with the given tenantID.
	// If the trust is already blocked, it does nothing.
	// Returns an error if the trust cannot be retrieved or updated.
	Block(ctx context.Context, tenantID string) error
	// Remove removes the trust for the given tenantID.
	Remove(ctx context.Context, tenantID string) error
	// Unblock sets the Blocked flag to false for the trust associated with the given tenantID.
	// If the trust is not blocked, it does nothing.
	// Returns an error if the trust cannot be retrieved or updated.
	Unblock(ctx context.Context, tenantID string) error
	// Get returns a trust message with optional extensions set.
	Get(ctx context.Context, tenantID string) (*trustv1.Trust, error)
}
