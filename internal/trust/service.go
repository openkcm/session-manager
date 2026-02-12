package trust

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/session-manager/internal/serviceerr"
)

type Service struct {
	repository OIDCMappingRepository
}

func NewService(repo OIDCMappingRepository) *Service {
	return &Service{
		repository: repo,
	}
}

func (s *Service) ApplyMapping(ctx context.Context, tenantID string, mapping OIDCMapping) error {
	_, err := s.repository.Get(ctx, tenantID)
	if err != nil {
		err = s.repository.Create(ctx, tenantID, mapping)
		if err != nil {
			return fmt.Errorf("creating mapping for tenant: %w", err)
		}
	} else {
		err = s.repository.Update(ctx, tenantID, mapping)
		if err != nil {
			return fmt.Errorf("updating mapping for tenant: %w", err)
		}
	}

	return nil
}

// BlockMapping sets the Blocked flag to true for the OIDC mapping associated with the given tenantID.
// If the mapping is already blocked, it does nothing.
// Returns an error if the mapping cannot be retrieved or updated.
func (s *Service) BlockMapping(ctx context.Context, tenantID string) error {
	mapping, err := s.repository.Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("getting mapping for tenant: %w", err)
	}
	if mapping.Blocked {
		return nil
	}
	mapping.Blocked = true
	err = s.repository.Update(ctx, tenantID, mapping)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("updating mapping for blocking tenant: %w", err)
	}
	return nil
}

func (s *Service) RemoveMapping(ctx context.Context, tenantID string) error {
	err := s.repository.Delete(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("deleting mapping for tenant: %w", err)
	}

	return nil
}

// UnblockMapping sets the Blocked flag to false for the OIDC mapping associated with the given tenantID.
// If the mapping is not blocked, it does nothing.
// Returns an error if the mapping cannot be retrieved or updated.
func (s *Service) UnblockMapping(ctx context.Context, tenantID string) error {
	mapping, err := s.repository.Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("getting mapping for tenant: %w", err)
	}
	if !mapping.Blocked {
		return nil
	}
	mapping.Blocked = false
	err = s.repository.Update(ctx, tenantID, mapping)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("updating mapping for unblocking tenant: %w", err)
	}
	return nil
}
