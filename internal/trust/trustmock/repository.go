package trustmock

import (
	"context"

	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/trust"
)

type RepositoryOption func(*Repository)

type Repository struct {
	tenantTrust map[string]trust.OIDCMapping

	getErr, createErr, deleteErr, updateErr error
}

func WithTrust(tenantID string, provider trust.OIDCMapping) RepositoryOption {
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
		tenantTrust: make(map[string]trust.OIDCMapping),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// TAdd is a helper method for tests to add a trust relationship.
func (r *Repository) TAdd(tenantID string, provider trust.OIDCMapping) {
	r.tenantTrust[tenantID] = provider
}

// TGet is a helper method for tests to get a trust relationship.
func (r *Repository) TGet(tenantID string) trust.OIDCMapping {
	return r.tenantTrust[tenantID]
}

func (r *Repository) Get(_ context.Context, tenantID string) (trust.OIDCMapping, error) {
	if r.getErr != nil {
		return trust.OIDCMapping{}, r.getErr
	}
	if provider, ok := r.tenantTrust[tenantID]; ok {
		return provider, nil
	}
	return trust.OIDCMapping{}, serviceerr.ErrNotFound
}

func (r *Repository) Create(_ context.Context, tenantID string, provider trust.OIDCMapping) error {
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

func (r *Repository) Update(_ context.Context, tenantID string, provider trust.OIDCMapping) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.tenantTrust[tenantID] = provider
	return nil
}
