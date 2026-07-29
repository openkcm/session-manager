package mocktrust

import (
	"context"

	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	"github.com/openkcm/session-manager/modules/oidctrust"
	"github.com/openkcm/session-manager/pkg/serviceerr"
)

type RepositoryOption func(*Repository)

type Repository struct {
	tenantTrust map[string]*trustv1.Trust

	getErr, createErr, deleteErr, updateErr error
}

func WithTrust(mapping *trustv1.Trust) RepositoryOption {
	return func(r *Repository) { r.tenantTrust[mapping.GetTenantId()] = mapping }
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

var _ oidctrust.TrustRepository = (*Repository)(nil)

func NewInMemRepository(opts ...RepositoryOption) *Repository {
	r := &Repository{
		tenantTrust: make(map[string]*trustv1.Trust),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// TAdd is a helper method for tests to add a trust relationship.
func (r *Repository) TAdd(mapping *trustv1.Trust) {
	r.tenantTrust[mapping.GetTenantId()] = mapping
}

// TGet is a helper method for tests to get a trust relationship.
func (r *Repository) TGet(tenantID string) *trustv1.Trust {
	return r.tenantTrust[tenantID]
}

func (r *Repository) Get(_ context.Context, tenantID string) (*trustv1.Trust, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if mapping, ok := r.tenantTrust[tenantID]; ok {
		return mapping, nil
	}
	return nil, serviceerr.ErrNotFound
}

func (r *Repository) Create(_ context.Context, mapping *trustv1.Trust) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.tenantTrust[mapping.GetTenantId()] = mapping
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

func (r *Repository) Update(_ context.Context, mapping *trustv1.Trust) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.tenantTrust[mapping.GetTenantId()] = mapping
	return nil
}
