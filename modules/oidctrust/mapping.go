package oidctrust

import (
	"context"
	"errors"
	"fmt"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	"github.com/openkcm/session-manager/pkg/serviceerr"
)

// ApplyMapping applies and stores the provided Trust.
func (m *TrustModule) ApplyMapping(ctx context.Context, trust *trustv1.Trust) error {
	if _, err := m.repository.Get(ctx, trust.GetTenantId()); err != nil {
		err = m.repository.Create(ctx, trust)
		if err != nil {
			return fmt.Errorf("creating mapping for tenant: %w", err)
		}
	} else {
		err = m.repository.Update(ctx, trust)
		if err != nil {
			return fmt.Errorf("updating mapping for tenant: %w", err)
		}
	}

	return nil
}

// BlockMapping sets the Blocked flag to true for the OIDC mapping associated with the given tenantID.
// If the mapping is already blocked, it does nothing.
// Returns an error if the mapping cannot be retrieved or updated.
func (m *TrustModule) BlockMapping(ctx context.Context, tenantID string) error {
	trust, err := m.repository.Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("getting mapping for tenant: %w", err)
	}
	if trust.GetBlocked() {
		return nil
	}

	trust.SetBlocked(true)
	if err = m.repository.Update(ctx, trust); err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("updating mapping for blocking tenant: %w", err)
	}
	return nil
}

func (m *TrustModule) RemoveMapping(ctx context.Context, tenantID string) error {
	err := m.repository.Delete(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("deleting mapping for tenant: %w", err)
	}

	return nil
}

// UnblockMapping sets the Blocked flag to false for the OIDC mapping associated with the given tenantID.
// If the mapping is not blocked, it does nothing.
// Returns an error if the mapping cannot be retrieved or updated.
func (m *TrustModule) UnblockMapping(ctx context.Context, tenantID string) error {
	trust, err := m.repository.Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("getting mapping for tenant: %w", err)
	}
	if !trust.GetBlocked() {
		return nil
	}
	trust.SetBlocked(false)
	if err = m.repository.Update(ctx, trust); err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("updating mapping for unblocking tenant: %w", err)
	}
	return nil
}

// Get returns a trust message with optional extensions set.
func (m *TrustModule) Get(ctx context.Context, tenantID string) (*trustv1.Trust, error) {
	trust, err := m.repository.Get(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting trust from repository: %w", err)
	}

	m.resolveExtensions(trust)
	return trust, nil
}

// resolveExtensions sets optional extensions to the Trust message and its details if configured.
func (m *TrustModule) resolveExtensions(trust *trustv1.Trust) {
	switch trust.WhichDetails() {
	case trustv1.Trust_Oidc_case:
		m.resolveOIDCExtensions(trust.GetOidc())
	}
}

// resolveOIDCExtensions sets optional extensions to the ODIC message if configured.
func (m *TrustModule) resolveOIDCExtensions(oidc *oidcv1.OIDC) {
}
