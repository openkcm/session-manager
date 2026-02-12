package oidcmock

import (
	"context"

	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/trust"
)

type RepositoryOption func(*Repository)

type Repository struct {
	tenantTrust map[string]trust.Provider

	getErr, createErr, deleteErr, updateErr error
}

func WithTrust(tenantID string, provider trust.Provider) RepositoryOption {
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

var _ = trust.ProviderRepository(&Repository{})

func NewInMemRepository(opts ...RepositoryOption) *Repository {
	r := &Repository{
		tenantTrust: make(map[string]trust.Provider),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// TAdd is a helper method for tests to add a trust relationship.
func (r *Repository) TAdd(tenantID string, provider trust.Provider) {
	r.tenantTrust[tenantID] = provider
}

// TGet is a helper method for tests to get a trust relationship.
func (r *Repository) TGet(tenantID string) trust.Provider {
	return r.tenantTrust[tenantID]
}

func (r *Repository) Get(_ context.Context, tenantID string) (trust.Provider, error) {
	if r.getErr != nil {
		return trust.Provider{}, r.getErr
	}
	if provider, ok := r.tenantTrust[tenantID]; ok {
		return provider, nil
	}
	return trust.Provider{}, serviceerr.ErrNotFound
}

func (r *Repository) Create(_ context.Context, tenantID string, provider trust.Provider) error {
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

func (r *Repository) Update(_ context.Context, tenantID string, provider trust.Provider) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.tenantTrust[tenantID] = provider
	return nil
}
