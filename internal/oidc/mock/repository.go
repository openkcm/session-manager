package oidcmock

import (
	"context"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

type Repository struct {
	Providers         map[string]oidc.Provider
	ProvidersToTenant map[string]oidc.Provider

	getErr, getForTenantErr, createErr, deleteErr, updateErr error
}

func NewInMemRepository(getErr, getForTenantErr, createErr, deleteErr, updateErr error) *Repository {
	return &Repository{
		Providers:         make(map[string]oidc.Provider),
		ProvidersToTenant: make(map[string]oidc.Provider),

		getErr:          getErr,
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

func (r *Repository) Get(ctx context.Context, issuerURL string) (oidc.Provider, error) {
	if r.getErr != nil {
		return oidc.Provider{}, r.getErr
	}

	if provider, ok := r.Providers[issuerURL]; ok {
		return provider, nil
	}

	return oidc.Provider{}, nil
}

func (r *Repository) Add(tenantID string, provider oidc.Provider) {
	r.Providers[provider.IssuerURL] = provider
	r.ProvidersToTenant[tenantID] = provider
}

func (r *Repository) Create(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if r.createErr != nil {
		return r.createErr
	}

	r.Add(tenantID, provider)

	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}

	if _, ok := r.Providers[provider.IssuerURL]; !ok {
		return serviceerr.ErrNotFound
	}

	delete(r.ProvidersToTenant, tenantID)

	return nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, provider oidc.Provider) error {
	if r.updateErr != nil {
		return r.updateErr
	}

	r.Providers[provider.IssuerURL] = provider
	r.ProvidersToTenant[tenantID] = provider

	return nil
}
