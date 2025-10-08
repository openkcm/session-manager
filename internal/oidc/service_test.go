package oidc_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/oidc"
	oidcmock "github.com/openkcm/session-manager/internal/oidc/mock"
)

var oidcProvider oidc.Provider
var newOIDCRepo func(getErr, getForTenantErr, createErr, deleteErr, updateErr error) *oidcmock.Repository

const (
	requestURI = "http://cmk.example.com/ui"
	issuerURL  = "http://oidc.example.com"
	tenantID   = "tenant-id"
)

func TestMain(m *testing.M) {
	oidcProvider = oidc.Provider{
		IssuerURL: issuerURL,
		Blocked:   false,
		JWKSURIs:  []string{"http://jwks.example.com"},
		Audiences: []string{requestURI},
	}
	newOIDCRepo = func(getErr, getForTenantErr, createErr, deleteErr, updateErr error) *oidcmock.Repository {
		oidcRepo := oidcmock.NewInMemRepository(getErr, getForTenantErr, createErr, deleteErr, updateErr)
		oidcRepo.Add(tenantID, oidcProvider)

		return oidcRepo
	}
	code := m.Run()
	os.Exit(code)
}

func TestService_Get(t *testing.T) {
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

func TestService_GetForTenant(t *testing.T) {
	tests := []struct {
		name         string
		tenantID     string
		oidcRepo     *oidcmock.Repository
		wantProvider oidc.Provider
		assertErr    assert.ErrorAssertionFunc
	}{
		{
			name:         "Success",
			tenantID:     tenantID,
			oidcRepo:     newOIDCRepo(nil, nil, nil, nil, nil),
			wantProvider: oidcProvider,
			assertErr:    assert.NoError,
		},
		{
			name:      "GetProviderForTenant OIDC error",
			oidcRepo:  newOIDCRepo(nil, errors.New("Repository.GetProviderForTenant() error"), nil, nil, nil),
			tenantID:  "does-not-exist",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oidc.NewService(tt.oidcRepo)

			gotProvider, err := s.GetProviderForTenant(t.Context(), tt.tenantID)
			if !tt.assertErr(t, err, fmt.Sprintf("Service.GetForTenant() error %v", err)) || err != nil {
				assert.Zerof(t, gotProvider, "Service.GetForTenant() extected zero value if an error is returned, got %v", gotProvider)
				return
			}

			assert.Equal(t, tt.wantProvider, gotProvider, "Service.GetForTenant()")
		})
	}
}

func TestService_Create(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		oidcRepo  *oidcmock.Repository
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "Success",
			tenantID:  tenantID,
			oidcRepo:  newOIDCRepo(nil, nil, nil, nil, nil),
			assertErr: assert.NoError,
		},
		{
			name:      "CreateProviderForTenant OIDC error",
			oidcRepo:  newOIDCRepo(nil, nil, errors.New("Repository.CreateProviderForTenant() error"), nil, nil),
			tenantID:  tenantID,
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oidc.NewService(tt.oidcRepo)

			err := s.CreateProviderForTenant(t.Context(), tt.tenantID, oidcProvider)
			if !tt.assertErr(t, err, fmt.Sprintf("Service.Create() error %v", err)) || err != nil {
				return
			}
		})
	}
}

func TestService_Delete(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		oidcRepo  *oidcmock.Repository
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "Success",
			tenantID:  tenantID,
			oidcRepo:  newOIDCRepo(nil, nil, nil, nil, nil),
			assertErr: assert.NoError,
		},
		{
			name:      "DeleteProviderForTenant OIDC error",
			oidcRepo:  newOIDCRepo(nil, nil, nil, errors.New("Repository.DeleteProviderForTenant() error"), nil),
			tenantID:  "does-not-exist",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oidc.NewService(tt.oidcRepo)

			err := s.DeleteProviderForTenant(t.Context(), tt.tenantID, oidcProvider)
			if !tt.assertErr(t, err, fmt.Sprintf("Service.Delete() error %v", err)) || err != nil {
				return
			}
		})
	}
}

func TestService_Update(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		oidcRepo  *oidcmock.Repository
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "Success",
			tenantID:  tenantID,
			oidcRepo:  newOIDCRepo(nil, nil, nil, nil, nil),
			assertErr: assert.NoError,
		},
		{
			name:      "UpdateProviderForTenant OIDC error",
			oidcRepo:  newOIDCRepo(nil, nil, nil, nil, errors.New("Repository.UpdateProviderForTenant() error")),
			tenantID:  "does-not-exist",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oidc.NewService(tt.oidcRepo)

			err := s.UpdateProviderForTenant(t.Context(), tt.tenantID, oidcProvider)
			if !tt.assertErr(t, err, fmt.Sprintf("Service.Update() error %v", err)) || err != nil {
				return
			}
		})
	}
}
