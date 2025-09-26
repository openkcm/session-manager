package oidc

import (
	"context"
	"time"
)

// ProviderRepository allows to read OIDC provider data for a tenant stored in the context.
type ProviderRepository interface {
	GetForTenant(ctx context.Context, tenantID string) (Provider, error)
	Create(ctx context.Context, tenantID string, provider Provider) error
	Delete(ctx context.Context, tenantID string, provider Provider) error
	Update(ctx context.Context, tenantID string, provider Provider) error
}

// TokenResponse represents the result of a token refresh operation.
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// TokenRefresher defines the interface for refreshing tokens.
type TokenRefresher interface {
	RefreshToken(ctx context.Context, tenantID, refreshToken string) (TokenResponse, error)
}
