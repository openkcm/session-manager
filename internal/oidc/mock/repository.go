package oidcmock

import (
	"context"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

type Repository struct {
	ProvidersToTenant map[string]oidc.Provider

	getForTenantErr, createErr, deleteErr, updateErr error
}

func NewInMemRepository(getForTenantErr, createErr, deleteErr, updateErr error) *Repository {
	return &Repository{
		ProvidersToTenant: make(map[string]oidc.Provider),

		getForTenantErr: getForTenantErr,
		createErr:       createErr,
		deleteErr:       deleteErr,
		updateErr:       updateErr,
	}
}

func (r *Repository) Get(ctx context.Context, tenantID string) (oidc.Provider, error) {
	if r.getForTenantErr != nil {
		return oidc.Provider{}, r.getForTenantErr
	}

	if provider, ok := r.ProvidersToTenant[tenantID]; ok {
		return provider, nil
	}

	return oidc.Provider{}, nil
}

func (r *Repository) Add(tenantID string, provider oidc.Provider) {
	r.ProvidersToTenant[tenantID] = provider
}

func (r *Repository) Create(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if r.createErr != nil {
		return r.createErr
	}

	r.Add(tenantID, provider)

	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}

	if _, ok := r.ProvidersToTenant[tenantID]; !ok {
		return serviceerr.ErrNotFound
	}

	delete(r.ProvidersToTenant, tenantID)

	return nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if r.updateErr != nil {
		return r.updateErr
	}

	r.ProvidersToTenant[tenantID] = provider

	return nil
}
