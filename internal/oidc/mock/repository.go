package oidcmock

import (
	"context"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

type Repository struct {
	set               map[string]struct{}
	ProvidersToTenant map[string]oidc.Provider

	getForTenantErr, createErr, deleteErr, updateErr error
}

func NewInMemRepository(getForTenantErr, createErr, deleteErr, updateErr error) *Repository {
	return &Repository{
		set:               make(map[string]struct{}),
		ProvidersToTenant: make(map[string]oidc.Provider),

		getForTenantErr: getForTenantErr,
		createErr:       createErr,
		deleteErr:       deleteErr,
		updateErr:       updateErr,
	}
}

func (r *Repository) GetForTenant(ctx context.Context, tenantID string) (oidc.Provider, error) {
	if r.getForTenantErr != nil {
		return oidc.Provider{}, r.getForTenantErr
	}

	if provider, ok := r.ProvidersToTenant[tenantID]; ok {
		return provider, nil
	}

	return oidc.Provider{}, nil
}

func (r *Repository) Create(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if r.createErr != nil {
		return r.createErr
	}

	r.set[provider.IssuerURL] = struct{}{}
	r.ProvidersToTenant[tenantID] = provider

	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}

	if _, ok := r.set[provider.IssuerURL]; !ok {
		return serviceerr.ErrNotFound
	}

	delete(r.ProvidersToTenant, tenantID)

	return nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if r.updateErr != nil {
		return r.updateErr
	}

	r.set[provider.IssuerURL] = struct{}{}
	r.ProvidersToTenant[tenantID] = provider

	return nil
}
