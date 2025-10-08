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
