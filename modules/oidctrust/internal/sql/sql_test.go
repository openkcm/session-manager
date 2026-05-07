package sqltrust_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/dbtest/postgrestest"
	sqltrust "github.com/openkcm/session-manager/modules/oidctrust/internal/sql"
	"github.com/openkcm/session-manager/pkg/serviceerr"
)

var dbPool sessionmanager.Database

type pooldb struct {
	*pgxpool.Pool
}

func (p *pooldb) STDAdapter() *sql.DB {
	return stdlib.OpenDBFromPool(p.Pool)
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	pool, _, terminate := postgrestest.Start(ctx)
	defer terminate(ctx)

	dbPool = &pooldb{pool}

	code := m.Run()
	os.Exit(code)
}

func TestRepository_Get(t *testing.T) {
	tests := []struct {
		name        string
		tenantID    string
		wantMapping *trustv1.Trust
		assertErr   assert.ErrorAssertionFunc
	}{
		{
			name:        "Success",
			tenantID:    "tenant1-id",
			wantMapping: trustv1.Trust_builder{TenantId: new("tenant1-id"), Blocked: new(false), Oidc: oidcv1.OIDC_builder{Issuer: new("url-one"), Audiences: make([]string, 0)}.Build()}.Build(),
			assertErr:   assert.NoError,
		},
		{
			name:      "Error does not exist",
			tenantID:  "does-not-exist",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sqltrust.NewRepository(dbPool)

			gotMapping, err := r.Get(t.Context(), tt.tenantID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Get() error %v", err)) || err != nil {
				assert.Zerof(t, gotMapping, "Repository.Get() extected zero value if an error is returned, got %v", gotMapping)
				return
			}

			if diff := cmp.Diff(tt.wantMapping, gotMapping, protocmp.Transform()); diff != "" {
				t.Fatalf("mapping not equal:\n%s", diff)
			}
		})
	}
}

func TestRepository_Create(t *testing.T) {
	tests := []struct {
		name      string
		mapping   *trustv1.Trust
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "Create succeeds",
			mapping:   trustv1.Trust_builder{TenantId: new("tenant-id-create-success"), Blocked: new(false), Oidc: oidcv1.OIDC_builder{Issuer: new("http://oidc-success.example.com"), JwksUri: new("jwks.example.com"), Audiences: []string{"cmk.example.com"}}.Build()}.Build(),
			assertErr: assert.NoError,
		},
		{
			name:      "Duplicate",
			mapping:   trustv1.Trust_builder{TenantId: new("tenant1-id"), Blocked: new(false), Oidc: oidcv1.OIDC_builder{Issuer: new("url-one"), JwksUri: new("jwks.example.com"), Audiences: []string{"cmk.example.com"}}.Build()}.Build(),
			assertErr: assert.Error,
		},
		{
			name:      "Create without JWKSURI and Audiences succeeds",
			mapping:   trustv1.Trust_builder{TenantId: new("tenant-id-create-without-jwks-aud-success"), Blocked: new(false), Oidc: oidcv1.OIDC_builder{Issuer: new("http://oidc-success-2.example.com"), Audiences: []string{}}.Build()}.Build(),
			assertErr: assert.NoError,
		},
		{
			name:      "Create without JWKSURI succeeds",
			mapping:   trustv1.Trust_builder{TenantId: new("tenant-id-create-without-jwks-success"), Blocked: new(false), Oidc: oidcv1.OIDC_builder{Issuer: new("http://oidc-success-3.example.com"), Audiences: []string{"cmk.example.com"}}.Build()}.Build(),
			assertErr: assert.NoError,
		},
		{
			name:      "Create without Audiences succeeds",
			mapping:   trustv1.Trust_builder{TenantId: new("tenant-id-create-without-aud-success"), Blocked: new(false), Oidc: oidcv1.OIDC_builder{Issuer: new("http://oidc-success-4.example.com"), JwksUri: new("jwks.example.com"), Audiences: []string{}}.Build()}.Build(),
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			r := sqltrust.NewRepository(dbPool)

			// When
			err := r.Create(t.Context(), tt.mapping)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Create() error %v", err)) || err != nil {
				return
			}

			// Then
			mapping, err := r.Get(t.Context(), tt.mapping.GetTenantId())
			require.NoError(t, err)

			if diff := cmp.Diff(tt.mapping, mapping, protocmp.Transform()); diff != "" {
				t.Fatalf("Unexpected mapping in the database (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestRepository_Delete(t *testing.T) {
	const tenantID = "tenant-id-delete-success"
	mapping := trustv1.Trust_builder{TenantId: new(tenantID), Blocked: new(false), Oidc: oidcv1.OIDC_builder{Issuer: new("http://oidc-to-delete.example.com"), JwksUri: new("jwks.example.com"), Audiences: []string{"cmk.example.com"}}.Build()}.Build()
	r := sqltrust.NewRepository(dbPool)
	err := r.Create(t.Context(), mapping)
	require.NoError(t, err, "Inserting test data")

	tests := []struct {
		name      string
		tenantID  string
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "Delete tenant",
			tenantID:  tenantID,
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
	mapping := trustv1.Trust_builder{TenantId: new(tenantID), Blocked: new(false), Oidc: oidcv1.OIDC_builder{Issuer: new("http://oidc-to-update.example.com"), JwksUri: new("jwks.example.com"), Audiences: []string{"cmk.example.com"}}.Build()}.Build()
	r := sqltrust.NewRepository(dbPool)
	err := r.Create(t.Context(), mapping)
	require.NoError(t, err, "Inserting test data")

	tests := []struct {
		name      string
		mapping   *trustv1.Trust
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "Update succeeds",
			mapping:   trustv1.Trust_builder{TenantId: new(tenantID), Blocked: new(true), Oidc: oidcv1.OIDC_builder{Issuer: new(mapping.GetOidc().GetIssuer()), JwksUri: new("jwks-updated.example.com"), Audiences: mapping.GetOidc().GetAudiences()}.Build()}.Build(),
			assertErr: assert.NoError,
		},
		{
			name:      "Does not exist",
			mapping:   trustv1.Trust_builder{TenantId: new("does-not-exist"), Blocked: new(true), Oidc: oidcv1.OIDC_builder{Issuer: new("does-not-exist"), JwksUri: new("jwks-updated.example.com"), Audiences: mapping.GetOidc().GetAudiences()}.Build()}.Build(),
			assertErr: assert.Error,
		},
		{
			name:      "Update without JWKSURI and Audiences succeeds",
			mapping:   trustv1.Trust_builder{TenantId: new(tenantID), Blocked: new(true), Oidc: oidcv1.OIDC_builder{Issuer: new(mapping.GetOidc().GetIssuer()), Audiences: []string{}}.Build()}.Build(),
			assertErr: assert.NoError,
		},
		{
			name:      "Update without JWKSURI succeeds",
			mapping:   trustv1.Trust_builder{TenantId: new(tenantID), Blocked: new(true), Oidc: oidcv1.OIDC_builder{Issuer: new(mapping.GetOidc().GetIssuer()), Audiences: mapping.GetOidc().GetAudiences()}.Build()}.Build(),
			assertErr: assert.NoError,
		},
		{
			name:      "Update without Audiences succeeds",
			mapping:   trustv1.Trust_builder{TenantId: new(tenantID), Blocked: new(true), Oidc: oidcv1.OIDC_builder{Issuer: new(mapping.GetOidc().GetIssuer()), JwksUri: new("jwks-updated.example.com"), Audiences: []string{}}.Build()}.Build(),
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Update(t.Context(), tt.mapping)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.Update() error %v", err)) || err != nil {
				return
			}

			gotMapping, err := r.Get(t.Context(), tt.mapping.GetTenantId())
			require.NoError(t, err)

			if diff := cmp.Diff(tt.mapping, gotMapping, protocmp.Transform()); diff != "" {
				t.Fatalf("Unexpected mapping in the database (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestPgTextOrNull(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  pgtype.Text
	}{
		{
			name:  "empty string returns invalid (null)",
			input: "",
			want:  pgtype.Text{String: "", Valid: false},
		},
		{
			name:  "non-empty string returns valid text",
			input: "hello",
			want:  pgtype.Text{String: "hello", Valid: true},
		},
		{
			name:  "whitespace-only string returns valid text",
			input: "   ",
			want:  pgtype.Text{String: "   ", Valid: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sqltrust.PgTextOrNull(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandlePgError(t *testing.T) {
	otherPgErr := &pgconn.PgError{Code: "42P01"} // undefined_table
	sentinel := errors.New("some other error")

	tests := []struct {
		name        string
		err         error
		wantErr     error
		wantHandled bool
	}{
		{
			name:        "duplicate key violation (23505) returns ErrConflict",
			err:         &pgconn.PgError{Code: "23505"},
			wantErr:     serviceerr.ErrConflict,
			wantHandled: true,
		},
		{
			name:        "other pg error code returns original error",
			err:         otherPgErr,
			wantErr:     otherPgErr,
			wantHandled: false,
		},
		{
			name:        "non-pg error returns original error",
			err:         sentinel,
			wantErr:     sentinel,
			wantHandled: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, handled := sqltrust.HandlePgError(tt.err)
			if handled != tt.wantHandled {
				t.Errorf("handled = %v, want %v", handled, tt.wantHandled)
			}
			if !errors.Is(got, tt.wantErr) {
				t.Errorf("err = %v, want %v", got, tt.wantErr)
			}
		})
	}
}
