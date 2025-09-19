package oidcsql_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/dbtest"
	"github.com/openkcm/session-manager/internal/oidc"
	oidcsql "github.com/openkcm/session-manager/internal/oidc/sql"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

var dbPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pool, _, terminate := dbtest.Start(ctx)
	defer terminate(ctx)

	dbPool = pool

	code := m.Run()
	os.Exit(code)
}

func TestRepository_GetForTenant(t *testing.T) {
	tests := []struct {
		name         string
		tenantID     string
		wantProvider oidc.Provider
		assertErr    assert.ErrorAssertionFunc
	}{
		{
			name:     "Success",
			tenantID: "tenant1-id",
			wantProvider: oidc.Provider{
				IssuerURL: "url-one",
				Blocked:   false,
				JWKSURIs:  make([]string, 0),
				Audiences: make([]string, 0),
			},
			assertErr: assert.NoError,
		},
		{
			name:      "Error does not exist",
			tenantID:  "does-not-exist",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := oidcsql.NewRepository(dbPool)

			gotProvider, err := r.GetForTenant(t.Context(), tt.tenantID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.GetForTenant() error %v", err)) || err != nil {
				return
			}

			assert.Equal(t, tt.wantProvider, gotProvider, "Repository.GetForTenant()")
		})
	}
}

func TestRepository_Create(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		provider  oidc.Provider
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:     "Create",
			tenantID: "tenant-id-create-success",
			provider: oidc.Provider{
				IssuerURL: "http://oidc-success.example.com",
				Blocked:   false,
				JWKSURIs:  []string{"jwks.example.com"},
				Audiences: []string{"cmk.example.com"},
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Duplicate",
			tenantID: "tenant1-id",
			provider: oidc.Provider{
				IssuerURL: "url-one",
				Blocked:   false,
				JWKSURIs:  []string{"jwks.example.com"},
				Audiences: []string{"cmk.example.com"},
			},
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := oidcsql.NewRepository(dbPool)

			err := r.Create(t.Context(), tt.tenantID, tt.provider)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Create() error %v", err)) || err != nil {
				return
			}

			provider, err := r.GetForTenant(t.Context(), tt.tenantID)
			require.NoError(t, err)

			assert.Equal(t, tt.provider, provider, "Inserted provider does not match")
		})
	}
}

func TestRepository_Delete(t *testing.T) {
	const tenantID = "tenant-id-delete-success"

	provider := oidc.Provider{
		IssuerURL: "http://oidc-to-delete.example.com",
		Blocked:   false,
		JWKSURIs:  []string{"jwks.example.com"},
		Audiences: []string{"cmk.example.com"},
	}

	r := oidcsql.NewRepository(dbPool)
	err := r.Create(t.Context(), tenantID, provider)
	require.NoError(t, err, "Inserting test data")

	tests := []struct {
		name      string
		tenantID  string
		provider  oidc.Provider
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "Delete tenant",
			tenantID:  tenantID,
			provider:  provider,
			assertErr: assert.NoError,
		},
		{
			name:      "Error does not exist",
			tenantID:  "does-not-exist",
			provider:  oidc.Provider{IssuerURL: "does-not-exist"},
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Delete(t.Context(), tt.tenantID, tt.provider)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Delete() error %v", err)) || err != nil {
				return
			}

			_, err = r.GetForTenant(t.Context(), tt.tenantID)
			if !errors.Is(err, serviceerr.ErrNotFound) {
				t.Error("The provider is expected to be deleted")
			}
		})
	}
}

func TestRepository_Update(t *testing.T) {
	const tenantID = "tenant-id-update-success"

	provider := oidc.Provider{
		IssuerURL: "http://oidc-to-update.example.com",
		Blocked:   false,
		JWKSURIs:  []string{"jwks.example.com"},
		Audiences: []string{"cmk.example.com"},
	}

	r := oidcsql.NewRepository(dbPool)
	err := r.Create(t.Context(), tenantID, provider)
	require.NoError(t, err, "Inserting test data")

	tests := []struct {
		name      string
		tenantID  string
		provider  oidc.Provider
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:     "Success",
			tenantID: tenantID,
			provider: oidc.Provider{
				IssuerURL: provider.IssuerURL,
				Blocked:   true,
				JWKSURIs:  []string{"jwks-updated.example.com"},
				Audiences: append(provider.Audiences, "new-audience.example.com"),
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Does not exist",
			tenantID: "does-not-exist",
			provider: oidc.Provider{
				IssuerURL: "does-not-exist",
				Blocked:   true,
				JWKSURIs:  []string{"jwks-updated.example.com"},
				Audiences: append(provider.Audiences, "new-audience.example.com"),
			},
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Update(t.Context(), tt.tenantID, tt.provider)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Update() error %v", err)) || err != nil {
				return
			}

			provider, err := r.GetForTenant(t.Context(), tt.tenantID)
			require.NoError(t, err)

			assert.Equal(t, tt.provider, provider, "Inserted provider does not match")
		})
	}
}
