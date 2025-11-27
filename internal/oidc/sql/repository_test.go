package oidcsql_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/dbtest/postgrestest"
	"github.com/openkcm/session-manager/internal/oidc"
	oidcsql "github.com/openkcm/session-manager/internal/oidc/sql"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

var dbPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pool, _, terminate := postgrestest.Start(ctx)
	defer terminate(ctx)

	dbPool = pool

	code := m.Run()
	os.Exit(code)
}

func TestRepository_Get(t *testing.T) {
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
				IssuerURL:  "url-one",
				Blocked:    false,
				JWKSURI:    "",
				Audiences:  make([]string, 0),
				Properties: make(map[string]string),
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

			gotProvider, err := r.Get(t.Context(), tt.tenantID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Get() error %v", err)) || err != nil {
				assert.Zerof(t, gotProvider, "Repository.Get() extected zero value if an error is returned, got %v", gotProvider)
				return
			}

			assert.Equal(t, tt.wantProvider, gotProvider, "Repository.Get()")
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
			name:     "Create succeeds",
			tenantID: "tenant-id-create-success",
			provider: oidc.Provider{
				IssuerURL: "http://oidc-success.example.com",
				Blocked:   false,
				JWKSURI:   "jwks.example.com",
				Audiences: []string{"cmk.example.com"},
				Properties: map[string]string{
					"prop1": "prop1val",
				},
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Duplicate",
			tenantID: "tenant1-id",
			provider: oidc.Provider{
				IssuerURL: "url-one",
				Blocked:   false,
				JWKSURI:   "jwks.example.com",
				Audiences: []string{"cmk.example.com"},
				Properties: map[string]string{
					"prop1": "prop1val",
				},
			},
			assertErr: assert.Error,
		},
		{
			name:     "Create without JWKSURI and Audiences succeeds",
			tenantID: "tenant-id-create-without-jwks-aud-success",
			provider: oidc.Provider{
				IssuerURL: "http://oidc-success-2.example.com",
				Blocked:   false,
				Audiences: []string{},
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Create without JWKSURI succeeds",
			tenantID: "tenant-id-create-without-jwks-success",
			provider: oidc.Provider{
				IssuerURL: "http://oidc-success-3.example.com",
				Blocked:   false,
				Audiences: []string{"cmk.example.com"},
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Create without Audiences succeeds",
			tenantID: "tenant-id-create-without-aud-success",
			provider: oidc.Provider{
				IssuerURL: "http://oidc-success-4.example.com",
				Blocked:   false,
				JWKSURI:   "jwks.example.com",
				Audiences: []string{},
			},
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			r := oidcsql.NewRepository(dbPool)

			// When
			err := r.Create(t.Context(), tt.tenantID, tt.provider)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Create() error %v", err)) || err != nil {
				return
			}

			// Then
			provider, err := r.Get(t.Context(), tt.tenantID)
			require.NoError(t, err)

			if diff := cmp.Diff(tt.provider, provider); diff != "" {
				t.Fatalf("Unexpected provider in the database (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestRepository_Delete(t *testing.T) {
	const tenantID = "tenant-id-delete-success"

	provider := oidc.Provider{
		IssuerURL: "http://oidc-to-delete.example.com",
		Blocked:   false,
		JWKSURI:   "jwks.example.com",
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
			err := r.Delete(t.Context(), tt.tenantID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Delete() error %v", err)) || err != nil {
				return
			}

			p, err := r.Get(t.Context(), tt.tenantID)
			if !errors.Is(err, serviceerr.ErrNotFound) {
				t.Error("The provider is expected to be deleted")
			}
			assert.Zero(t, p, "The provider is expected to be deleted, instead a value is returned")
		})
	}
}

func TestRepository_Update(t *testing.T) {
	const tenantID = "tenant-id-update-success"

	provider := oidc.Provider{
		IssuerURL: "http://oidc-to-update.example.com",
		Blocked:   false,
		JWKSURI:   "jwks.example.com",
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
			name:     "Update succeeds",
			tenantID: tenantID,
			provider: oidc.Provider{
				IssuerURL: provider.IssuerURL,
				Blocked:   true,
				JWKSURI:   "jwks-updated.example.com",
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
				JWKSURI:   "jwks-updated.example.com",
				Audiences: append(provider.Audiences, "new-audience.example.com"),
			},
			assertErr: assert.Error,
		},
		{
			name:     "Update without JWKSURI and Audiences succeeds",
			tenantID: tenantID,
			provider: oidc.Provider{
				IssuerURL: provider.IssuerURL,
				Blocked:   true,
				Audiences: []string{},
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Update without JWKSURI succeeds",
			tenantID: tenantID,
			provider: oidc.Provider{
				IssuerURL: provider.IssuerURL,
				Blocked:   true,
				Audiences: append(provider.Audiences, "new-audience.example.com"),
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Update without Audiences succeeds",
			tenantID: tenantID,
			provider: oidc.Provider{
				IssuerURL: provider.IssuerURL,
				Blocked:   true,
				JWKSURI:   "jwks-updated.example.com",
				Audiences: []string{},
			},
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Update(t.Context(), tt.tenantID, tt.provider)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Update() error %v", err)) || err != nil {
				return
			}

			provider, err := r.Get(t.Context(), tt.tenantID)
			require.NoError(t, err)

			if diff := cmp.Diff(tt.provider, provider); diff != "" {
				t.Fatalf("Unexpected provider in the database (-want, +got):\n%s", diff)
			}
		})
	}
}
