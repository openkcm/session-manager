package oidc_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestService_RefreshToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test-access","refresh_token":"test-refresh","expires_in":3600}`)
	}))
	defer ts.Close()

	provider := oidc.Provider{IssuerURL: ts.URL + "/"}
	repo := &mockProviderRepository{provider: provider}
	svc := oidc.NewService(repo)

	resp, err := svc.RefreshToken(context.Background(), provider.IssuerURL, "dummy-refresh", "dummy-client")
	assert.NoError(t, err)
	assert.Equal(t, "test-access", resp.AccessToken)
	assert.Equal(t, "test-refresh", resp.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(3600*time.Second), resp.ExpiresAt, time.Second)
}

func TestService_RefreshToken_Non200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusBadRequest)
	}))
	defer ts.Close()

	provider := oidc.Provider{IssuerURL: ts.URL + "/"}
	repo := &mockProviderRepository{provider: provider}
	svc := oidc.NewService(repo)

	_, err := svc.RefreshToken(context.Background(), provider.IssuerURL, "dummy-refresh", "dummy-client")
	assert.Error(t, err)
}

func TestService_RefreshToken_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not-json`)
	}))
	defer ts.Close()

	provider := oidc.Provider{IssuerURL: ts.URL + "/"}
	repo := &mockProviderRepository{provider: provider}
	svc := oidc.NewService(repo)

	_, err := svc.RefreshToken(context.Background(), provider.IssuerURL, "dummy-refresh", "dummy-client")
	assert.Error(t, err)
}

type mockProviderRepository struct {
	provider oidc.Provider
}

func (m *mockProviderRepository) Get(ctx context.Context, issuer string) (oidc.Provider, error) {
	return m.provider, nil
}

func (m *mockProviderRepository) GetForTenant(ctx context.Context, tenantID string) (oidc.Provider, error) {
	return m.provider, nil
}

// Add stub Create method to satisfy interface
func (m *mockProviderRepository) Create(ctx context.Context, tenantID string, provider oidc.Provider) error {
	return nil
}

func (m *mockProviderRepository) Delete(ctx context.Context, tenantID string, provider oidc.Provider) error {
	return nil
}

func (m *mockProviderRepository) Update(ctx context.Context, tenantID string, provider oidc.Provider) error {
	return nil
}
