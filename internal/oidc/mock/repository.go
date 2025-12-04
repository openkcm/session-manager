package oidcmock

import (
	"context"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

type RepositoryOption func(*Repository)

type Repository struct {
	tenantTrust map[string]oidc.Provider

	getErr, createErr, deleteErr, updateErr error
}

func WithTrust(tenantID string, provider oidc.Provider) RepositoryOption {
	return func(r *Repository) { r.tenantTrust[tenantID] = provider }
}
func WithGetError(err error) RepositoryOption {
	return func(r *Repository) { r.getErr = err }
}
func WithCreateError(err error) RepositoryOption {
	return func(r *Repository) { r.createErr = err }
}
func WithDeleteError(err error) RepositoryOption {
	return func(r *Repository) { r.deleteErr = err }
}
func WithUpdateError(err error) RepositoryOption {
	return func(r *Repository) { r.updateErr = err }
}

var _ = oidc.ProviderRepository(&Repository{})

func NewInMemRepository(opts ...RepositoryOption) *Repository {
	r := &Repository{
		tenantTrust: make(map[string]oidc.Provider),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// TAdd is a helper method for tests to add a trust relationship.
func (r *Repository) TAdd(tenantID string, provider oidc.Provider) {
	r.tenantTrust[tenantID] = provider
}

// TGet is a helper method for tests to get a trust relationship.
func (r *Repository) TGet(tenantID string) oidc.Provider {
	return r.tenantTrust[tenantID]
}

func (r *Repository) Get(_ context.Context, tenantID string) (oidc.Provider, error) {
	if r.getErr != nil {
		return oidc.Provider{}, r.getErr
	}
	if provider, ok := r.tenantTrust[tenantID]; ok {
		return provider, nil
	}
	return oidc.Provider{}, serviceerr.ErrNotFound
}

func (r *Repository) Create(_ context.Context, tenantID string, provider oidc.Provider) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.tenantTrust[tenantID] = provider
	return nil
}

func (r *Repository) Delete(_ context.Context, tenantID string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.tenantTrust[tenantID]; !ok {
		return serviceerr.ErrNotFound
	}
	delete(r.tenantTrust, tenantID)
	return nil
}

func (r *Repository) Update(_ context.Context, tenantID string, provider oidc.Provider) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.tenantTrust[tenantID] = provider
	return nil
}
