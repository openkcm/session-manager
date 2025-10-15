package oidc

import (
	"context"
	"fmt"
)

type Service struct {
	repository ProviderRepository
}

func NewService(repo ProviderRepository) *Service {
	return &Service{
		repository: repo,
	}
}

func (s *Service) GetProvider(ctx context.Context, issuer string) (Provider, error) {
	provider, err := s.repository.Get(ctx, issuer)
	if err != nil {
		return Provider{}, fmt.Errorf("getting provider by issuer URL: %w", err)
	}

	return provider, nil
}

func (s *Service) ApplyMapping(ctx context.Context, tenantID string, provider Provider) error {
	_, err := s.repository.GetForTenant(ctx, tenantID)
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

func (s *Service) RemoveMapping(ctx context.Context, tenantID string) error {
	provider, err := s.repository.GetForTenant(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("getting provider for tenant: %w", err)
	}
	err = s.repository.Delete(ctx, tenantID, provider)
	if err != nil {
		return fmt.Errorf("deleting provider for tenant: %w", err)
	}

	return nil
}
