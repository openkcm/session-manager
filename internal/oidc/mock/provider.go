package oidcmock

import (
	"context"

	"github.com/openkcm/session-manager/internal/oidc"
)

// ProviderMock implements oidc.Provider interface for testing
// All methods are optional and can be set via function fields
// If not set, they return zero values

type ProviderMock struct {
	RefreshTokenFunc func(ctx context.Context, refreshToken string, clientID string, tokenEndpoint string) (oidc.TokenResponse, error)
	// Add other methods as needed for full Provider interface
}

func (m *ProviderMock) RefreshToken(ctx context.Context, refreshToken string, clientID string, tokenEndpoint string) (oidc.TokenResponse, error) {
	if m.RefreshTokenFunc != nil {
		return m.RefreshTokenFunc(ctx, refreshToken, clientID, tokenEndpoint)
	}
	return oidc.TokenResponse{}, nil
}

// ...implement other Provider methods as needed...
