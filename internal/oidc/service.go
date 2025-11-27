package oidc

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/session-manager/internal/serviceerr"
)

type Service struct {
	repository ProviderRepository
}

func NewService(repo ProviderRepository) *Service {
	return &Service{
		repository: repo,
	}
}

func (s *Service) ApplyMapping(ctx context.Context, tenantID string, provider Provider) error {
	_, err := s.repository.Get(ctx, tenantID)
	if err != nil {
		err = s.repository.Create(ctx, tenantID, provider)
		if err != nil {
			return fmt.Errorf("creating provider for tenant: %w", err)
		}
	} else {
		err = s.repository.Update(ctx, tenantID, provider)
		if err != nil {
			return fmt.Errorf("updating provider for tenant: %w", err)
		}
	}

	return nil
}

// BlockMapping sets the Blocked flag to true for the OIDC provider associated with the given tenantID.
// If the provider is already blocked, it does nothing.
// Returns an error if the provider cannot be retrieved or updated.
func (s *Service) BlockMapping(ctx context.Context, tenantID string) error {
	provider, err := s.repository.Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("getting provider for tenant: %w", err)
	}
	if provider.Blocked {
		return nil
	}
	provider.Blocked = true
	err = s.repository.Update(ctx, tenantID, provider)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("updating provider for blocking tenant: %w", err)
	}
	return nil
}

func (s *Service) RemoveMapping(ctx context.Context, tenantID string) error {
	if err := s.repository.Delete(ctx, tenantID); err != nil {
		return fmt.Errorf("deleting provider for tenant: %w", err)
	}

	return nil
}

// UnblockMapping sets the Blocked flag to false for the OIDC provider associated with the given tenantID.
// If the provider is not blocked, it does nothing.
// Returns an error if the provider cannot be retrieved or updated.
func (s *Service) UnblockMapping(ctx context.Context, tenantID string) error {
	provider, err := s.repository.Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("getting provider for tenant: %w", err)
	}
	if !provider.Blocked {
		return nil
	}
	provider.Blocked = false
	err = s.repository.Update(ctx, tenantID, provider)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("updating provider for blocking tenant: %w", err)
	}
	return nil
}
