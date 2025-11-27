package oidcsql

import (
	"context"
	"encoding/json"
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

func (r *Repository) Get(ctx context.Context, tenantID string) (oidc.Provider, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return oidc.Provider{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `SELECT issuer, blocked, jwks_uri, audiences, properties FROM trust WHERE tenant_id = $1;`, tenantID)
	return r.get(ctx, tx, row)
}

func (r *Repository) get(ctx context.Context, tx pgx.Tx, row pgx.Row) (oidc.Provider, error) {
	var propsBytes []byte
	var provider oidc.Provider
	if err := row.Scan(&provider.IssuerURL, &provider.Blocked, &provider.JWKSURI, &provider.Audiences, &propsBytes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return oidc.Provider{}, serviceerr.ErrNotFound
		}

		return oidc.Provider{}, fmt.Errorf("scanning rows: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return oidc.Provider{}, fmt.Errorf("committing tx: %w", err)
	}

	if len(propsBytes) > 0 {
		if err := json.Unmarshal(propsBytes, &provider.Properties); err != nil {
			return oidc.Provider{}, fmt.Errorf("unmarshalling properties: %w", err)
		}
	} else {
		provider.Properties = make(map[string]string)
	}

	return provider, nil
}

func (r *Repository) Create(ctx context.Context, tenantID string, provider oidc.Provider) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	propsBytes, err := r.marshalProperties(provider)
	if err != nil {
		return fmt.Errorf("marshaling properties: %w", err)
	}

	// The audiences value is optional, so we use COALESCE to default to an empty array if it's nil
	if _, err := tx.Exec(ctx,
		`INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties)
			 VALUES ($1, $2, $3, $4, COALESCE($5, '{}'::text[]), $6);`,
		tenantID, provider.Blocked, provider.IssuerURL, provider.JWKSURI, provider.Audiences, propsBytes,
	); err != nil {
		if err, ok := handlePgError(err); ok {
			return err
		}

		return fmt.Errorf("inserting into trust: %w", err)
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

	ct, err := tx.Exec(ctx, `DELETE FROM trust WHERE tenant_id = $1;`, tenantID)
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

	propsBytes, err := r.marshalProperties(provider)
	if err != nil {
		return err
	}

	// The audiences value is optional, so we use COALESCE to default to an empty array if it's nil
	ct, err := tx.Exec(ctx,
		`UPDATE trust
			 SET blocked = $1, issuer = $2, jwks_uri = $3, audiences = COALESCE($4, '{}'::text[]), properties = $5
			 WHERE tenant_id = $6;`,
		provider.Blocked, provider.IssuerURL, provider.JWKSURI, provider.Audiences, propsBytes, tenantID)
	if err != nil {
		return fmt.Errorf("updating trust: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return serviceerr.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing tx: %w", err)
	}

	return nil
}

func (r *Repository) marshalProperties(provider oidc.Provider) ([]byte, error) {
	propsBytes, err := json.Marshal(provider.Properties)
	if err != nil {
		return nil, fmt.Errorf("marshaling json: %w", err)
	}
	return propsBytes, nil
}
