package oidctrust

import (
	"context"
	"errors"
	"fmt"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	"github.com/openkcm/session-manager/pkg/serviceerr"
)

// Apply implements [sessionmanager.Trust].
func (m *TrustModule) Apply(ctx context.Context, trust *trustv1.Trust) error {
	if _, err := m.repository.Get(ctx, trust.GetTenantId()); err != nil {
		err = m.repository.Create(ctx, trust)
		if err != nil {
			return fmt.Errorf("creating trust for tenant: %w", err)
		}
	} else {
		err = m.repository.Update(ctx, trust)
		if err != nil {
			return fmt.Errorf("updating trust for tenant: %w", err)
		}
	}

	return nil
}

// Block implements [sessionmanager.Trust].
func (m *TrustModule) Block(ctx context.Context, tenantID string) error {
	trust, err := m.repository.Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("getting trust for tenant: %w", err)
	}
	if trust.GetBlocked() {
		return nil
	}

	trust.SetBlocked(true)
	if err = m.repository.Update(ctx, trust); err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("updating trust for blocking tenant: %w", err)
	}
	return nil
}

// Remove implements [sessionmanager.Trust].
func (m *TrustModule) Remove(ctx context.Context, tenantID string) error {
	err := m.repository.Delete(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("deleting trust for tenant: %w", err)
	}

	return nil
}

// Unblock implements [sessionmanager.Trust].
func (m *TrustModule) Unblock(ctx context.Context, tenantID string) error {
	trust, err := m.repository.Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("getting trust for tenant: %w", err)
	}
	if !trust.GetBlocked() {
		return nil
	}
	trust.SetBlocked(false)
	if err = m.repository.Update(ctx, trust); err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("updating trust for unblocking tenant: %w", err)
	}
	return nil
}

// Get implements [sessionmanager.Trust].
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
