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

func TestService_ApplyMapping(t *testing.T) {
	tests := []struct {
		name     string
		tenant   string
		oidcRepo *oidcmock.Repository
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "Success",
			tenant:   tenantID,
			oidcRepo: newOIDCRepo(nil, nil, nil, nil, nil),
			wantErr:  assert.NoError,
		},
		{
			name:     "Create error",
			tenant:   tenantID,
			oidcRepo: newOIDCRepo(nil, errors.New("getForTenant failed"), errors.New("create failed"), nil, nil),
			wantErr:  assert.Error,
		},
		{
			name:     "Update error",
			tenant:   tenantID,
			oidcRepo: newOIDCRepo(nil, nil, nil, nil, errors.New("update failed")),
			wantErr:  assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oidc.NewService(tt.oidcRepo)
			err := s.ApplyMapping(t.Context(), tt.tenant, oidcProvider)
			tt.wantErr(t, err)
		})
	}
}

func TestService_RemoveMapping(t *testing.T) {
	tests := []struct {
		name     string
		tenant   string
		oidcRepo *oidcmock.Repository
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "Success",
			tenant:   tenantID,
			oidcRepo: newOIDCRepo(nil, nil, nil, nil, nil),
			wantErr:  assert.NoError,
		},
		{
			name:     "GetForTenant error",
			tenant:   tenantID,
			oidcRepo: newOIDCRepo(nil, errors.New("getForTenant failed"), nil, nil, nil),
			wantErr:  assert.Error,
		},
		{
			name:     "Delete error",
			tenant:   tenantID,
			oidcRepo: newOIDCRepo(nil, nil, nil, errors.New("delete failed"), nil),
			wantErr:  assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oidc.NewService(tt.oidcRepo)
			err := s.RemoveMapping(t.Context(), tt.tenant)
			tt.wantErr(t, err)
		})
	}
}
