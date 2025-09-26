package oidcsql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func setTenantContext(ctx context.Context, tx pgx.Tx, tenantID string) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true);`, tenantID); err != nil {
		return fmt.Errorf("setting app.tenant_id: %w", err)
	}

	return nil
}

func (r *Repository) GetForTenant(ctx context.Context, tenantID string) (oidc.Provider, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return oidc.Provider{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := setTenantContext(ctx, tx, tenantID); err != nil {
		return oidc.Provider{}, fmt.Errorf("setting tenant context: %w", err)
	}

	var provider oidc.Provider
	if err := tx.QueryRow(
		ctx, `SELECT p.issuer_url, p.blocked, p.jwks_uris, p.audience
FROM oidc_providers p
	JOIN oidc_provider_map m ON m.issuer_url = p.issuer_url
WHERE m.tenant_id = current_setting('app.tenant_id');`).
		Scan(&provider.IssuerURL, &provider.Blocked, &provider.JWKSURIs, &provider.Audiences); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return oidc.Provider{}, serviceerr.ErrNotFound
		}

		return oidc.Provider{}, fmt.Errorf("selecting from oidc_providers: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return oidc.Provider{}, fmt.Errorf("committing tx: %w", err)
	}

	return provider, nil
}

func (r *Repository) Create(ctx context.Context, tenantID string, provider oidc.Provider) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	if err := setTenantContext(ctx, tx, tenantID); err != nil {
		return fmt.Errorf("setting tenant context: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO oidc_providers (issuer_url, blocked, jwks_uris, audience) VALUES ($1, $2, $3, $4);`,
		provider.IssuerURL, provider.Blocked, provider.JWKSURIs, provider.Audiences,
	); err != nil {
		if err, ok := handlePgError(err); ok {
			return err
		}

		return fmt.Errorf("inserting into oidc_providers: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO oidc_provider_map (tenant_id, issuer_url) VALUES (current_setting('app.tenant_id'), $1);`,
		provider.IssuerURL,
	); err != nil {
		if err, ok := handlePgError(err); ok {
			return err
		}

		return fmt.Errorf("inserting into oidc_provider_map: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID string, provider oidc.Provider) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := setTenantContext(ctx, tx, tenantID); err != nil {
		return fmt.Errorf("setting tenant context: %w", err)
	}

	ct, err := tx.Exec(ctx, `DELETE FROM oidc_providers WHERE issuer_url = $1;`, provider.IssuerURL)
	if err != nil {
		return fmt.Errorf("executing sql query: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return serviceerr.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing tx: %w", err)
	}

	return nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, provider oidc.Provider) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := setTenantContext(ctx, tx, tenantID); err != nil {
		return fmt.Errorf("setting tenant context: %w", err)
	}

	ct, err := tx.Exec(ctx, `UPDATE oidc_providers SET blocked = $1, jwks_uris = $2, audience = $3
WHERE issuer_url = $4;`, provider.Blocked, provider.JWKSURIs, provider.Audiences, provider.IssuerURL)
	if err != nil {
		return fmt.Errorf("updating oidc_providers: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return serviceerr.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing tx: %w", err)
	}

	return nil
}
