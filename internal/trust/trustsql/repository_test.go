package trustsql_test

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
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/trust"
	"github.com/openkcm/session-manager/internal/trust/trustsql"
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
		name        string
		tenantID    string
		wantMapping trust.OIDCMapping
		assertErr   assert.ErrorAssertionFunc
	}{
		{
			name:     "Success",
			tenantID: "tenant1-id",
			wantMapping: trust.OIDCMapping{
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
			r := trustsql.NewRepository(dbPool)

			gotMapping, err := r.Get(t.Context(), tt.tenantID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Get() error %v", err)) || err != nil {
				assert.Zerof(t, gotMapping, "Repository.Get() extected zero value if an error is returned, got %v", gotMapping)
				return
			}

			assert.Equal(t, tt.wantMapping, gotMapping, "Repository.Get()")
		})
	}
}

func TestRepository_Create(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		mapping   trust.OIDCMapping
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:     "Create succeeds",
			tenantID: "tenant-id-create-success",
			mapping: trust.OIDCMapping{
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
			mapping: trust.OIDCMapping{
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
			mapping: trust.OIDCMapping{
				IssuerURL: "http://oidc-success-2.example.com",
				Blocked:   false,
				Audiences: []string{},
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Create without JWKSURI succeeds",
			tenantID: "tenant-id-create-without-jwks-success",
			mapping: trust.OIDCMapping{
				IssuerURL: "http://oidc-success-3.example.com",
				Blocked:   false,
				Audiences: []string{"cmk.example.com"},
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Create without Audiences succeeds",
			tenantID: "tenant-id-create-without-aud-success",
			mapping: trust.OIDCMapping{
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
			r := trustsql.NewRepository(dbPool)

			// When
			err := r.Create(t.Context(), tt.tenantID, tt.mapping)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Create() error %v", err)) || err != nil {
				return
			}

			// Then
			mapping, err := r.Get(t.Context(), tt.tenantID)
			require.NoError(t, err)

			if diff := cmp.Diff(tt.mapping, mapping); diff != "" {
				t.Fatalf("Unexpected mapping in the database (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestRepository_Delete(t *testing.T) {
	const tenantID = "tenant-id-delete-success"

	mapping := trust.OIDCMapping{
		IssuerURL: "http://oidc-to-delete.example.com",
		Blocked:   false,
		JWKSURI:   "jwks.example.com",
		Audiences: []string{"cmk.example.com"},
	}

	r := trustsql.NewRepository(dbPool)
	err := r.Create(t.Context(), tenantID, mapping)
	require.NoError(t, err, "Inserting test data")

	tests := []struct {
		name      string
		tenantID  string
		mapping   trust.OIDCMapping
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "Delete tenant",
			tenantID:  tenantID,
			mapping:   mapping,
			assertErr: assert.NoError,
		},
		{
			name:      "Error does not exist",
			tenantID:  "does-not-exist",
			mapping:   trust.OIDCMapping{IssuerURL: "does-not-exist"},
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Delete(t.Context(), tt.tenantID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Delete() error %v", err)) || err != nil {
				return
			}

			gotMapping, err := r.Get(t.Context(), tt.tenantID)
			if !errors.Is(err, serviceerr.ErrNotFound) {
				t.Error("The mapping is expected to be deleted")
			}
			assert.Zero(t, gotMapping, "The mapping is expected to be deleted, instead a value is returned")
		})
	}
}

func TestRepository_Update(t *testing.T) {
	const tenantID = "tenant-id-update-success"

	mapping := trust.OIDCMapping{
		IssuerURL: "http://oidc-to-update.example.com",
		Blocked:   false,
		JWKSURI:   "jwks.example.com",
		Audiences: []string{"cmk.example.com"},
	}

	r := trustsql.NewRepository(dbPool)
	err := r.Create(t.Context(), tenantID, mapping)
	require.NoError(t, err, "Inserting test data")

	tests := []struct {
		name      string
		tenantID  string
		mapping   trust.OIDCMapping
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:     "Update succeeds",
			tenantID: tenantID,
			mapping: trust.OIDCMapping{
				IssuerURL: mapping.IssuerURL,
				Blocked:   true,
				JWKSURI:   "jwks-updated.example.com",
				Audiences: append(mapping.Audiences, "new-audience.example.com"),
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Does not exist",
			tenantID: "does-not-exist",
			mapping: trust.OIDCMapping{
				IssuerURL: "does-not-exist",
				Blocked:   true,
				JWKSURI:   "jwks-updated.example.com",
				Audiences: append(mapping.Audiences, "new-audience.example.com"),
			},
			assertErr: assert.Error,
		},
		{
			name:     "Update without JWKSURI and Audiences succeeds",
			tenantID: tenantID,
			mapping: trust.OIDCMapping{
				IssuerURL: mapping.IssuerURL,
				Blocked:   true,
				Audiences: []string{},
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Update without JWKSURI succeeds",
			tenantID: tenantID,
			mapping: trust.OIDCMapping{
				IssuerURL: mapping.IssuerURL,
				Blocked:   true,
				Audiences: append(mapping.Audiences, "new-audience.example.com"),
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Update without Audiences succeeds",
			tenantID: tenantID,
			mapping: trust.OIDCMapping{
				IssuerURL: mapping.IssuerURL,
				Blocked:   true,
				JWKSURI:   "jwks-updated.example.com",
				Audiences: []string{},
			},
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Update(t.Context(), tt.tenantID, tt.mapping)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Update() error %v", err)) || err != nil {
				return
			}

			gotMapping, err := r.Get(t.Context(), tt.tenantID)
			require.NoError(t, err)

			if diff := cmp.Diff(tt.mapping, gotMapping); diff != "" {
				t.Fatalf("Unexpected mapping in the database (-want, +got):\n%s", diff)
			}
		})
	}
}
