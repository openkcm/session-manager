package oidc_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/oidc"
	oidcmock "github.com/openkcm/session-manager/internal/oidc/mock"
)

func TestService_Get(t *testing.T) {
	const (
		requestURI = "http://cmk.example.com/ui"
		issuerURL  = "http://oidc.example.com"
		tenantID   = "tenant-id"
	)

	oidcProvider := oidc.Provider{
		IssuerURL: issuerURL,
		Blocked:   false,
		JWKSURIs:  []string{"http://jwks.example.com"},
		Audiences: []string{requestURI},
	}
	newOIDCRepo := func(getErr, getForTenantErr, createErr, deleteErr, updateErr error) *oidcmock.Repository {
		oidcRepo := oidcmock.NewInMemRepository(getErr, getForTenantErr, createErr, deleteErr, updateErr)
		oidcRepo.Add(tenantID, oidcProvider)

		return oidcRepo
	}

	tests := []struct {
		name         string
		issuerURL    string
		oidcRepo     *oidcmock.Repository
		wantProvider oidc.Provider
		assertErr    assert.ErrorAssertionFunc
	}{
		{
			name:         "Success",
			issuerURL:    issuerURL,
			oidcRepo:     newOIDCRepo(nil, nil, nil, nil, nil),
			wantProvider: oidcProvider,
			assertErr:    assert.NoError,
		},
		{
			name:      "Get OIDC error",
			oidcRepo:  newOIDCRepo(errors.New("Repository.Get() error"), nil, nil, nil, nil),
			issuerURL: "does-not-exist",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oidc.NewService(tt.oidcRepo)

			gotProvider, err := s.GetProvider(t.Context(), tt.issuerURL)
			if !tt.assertErr(t, err, fmt.Sprintf("Service.GetProvider() error %v", err)) || err != nil {
				assert.Zerof(t, gotProvider, "Service.GetProvider() extected zero value if an error is returned, got %v", gotProvider)
				return
			}

			assert.Equal(t, tt.wantProvider, gotProvider, "Service.GetProvider()")
		})
	}
}
