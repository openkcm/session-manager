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

func (s *Service) GetForTenant(ctx context.Context, tenantID string) (Provider, error) {
	provider, err := s.repository.GetForTenant(ctx, tenantID)
	if err != nil {
		return Provider{}, fmt.Errorf("getting provider by tenant URL: %w", err)
	}

	return provider, nil
}

func (s *Service) Create(ctx context.Context, tenantID string, provider Provider) error {
	err := s.repository.Create(ctx, tenantID, provider)
	if err != nil {
		return fmt.Errorf("creating provider: %w", err)
	}

	return nil
}

func (s *Service) Delete(ctx context.Context, tenantID string, provider Provider) error {
	err := s.repository.Delete(ctx, tenantID, provider)
	if err != nil {
		return fmt.Errorf("deleting provider: %w", err)
	}

	return nil
}

func (s *Service) Update(ctx context.Context, tenantID string, provider Provider) error {
	err := s.repository.Update(ctx, tenantID, provider)
	if err != nil {
		return fmt.Errorf("updating provider: %w", err)
	}

	return nil
}
