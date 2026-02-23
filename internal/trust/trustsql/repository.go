package trustsql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/trust"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) Get(ctx context.Context, tenantID string) (trust.OIDCMapping, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "get_oidc_mapping_sql")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		span.RecordError(err)
		return trust.OIDCMapping{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `SELECT issuer, blocked, jwks_uri, audiences, properties FROM trust WHERE tenant_id = $1;`, tenantID)
	mapping, err := r.get(ctx, tx, row)
	if err != nil {
		span.RecordError(err)
		return trust.OIDCMapping{}, err
	}

	return mapping, nil
}

func (r *Repository) get(ctx context.Context, tx pgx.Tx, row pgx.Row) (trust.OIDCMapping, error) {
	var propsBytes []byte
	var mapping trust.OIDCMapping

	err := row.Scan(&mapping.IssuerURL, &mapping.Blocked, &mapping.JWKSURI, &mapping.Audiences, &propsBytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return trust.OIDCMapping{}, serviceerr.ErrNotFound
		} else {
			return trust.OIDCMapping{}, fmt.Errorf("scanning rows: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return trust.OIDCMapping{}, fmt.Errorf("committing tx: %w", err)
	}

	if len(propsBytes) > 0 {
		err := json.Unmarshal(propsBytes, &mapping.Properties)
		if err != nil {
			return trust.OIDCMapping{}, fmt.Errorf("unmarshalling properties: %w", err)
		}
	} else {
		mapping.Properties = make(map[string]string)
	}

	return mapping, nil
}

func (r *Repository) Create(ctx context.Context, tenantID string, mapping trust.OIDCMapping) error {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "create_oidc_mapping_sql")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("starting transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	propsBytes, err := r.marshalProperties(mapping)
	if err != nil {
		return fmt.Errorf("marshaling properties: %w", err)
	}

	// The audiences value is optional, so we use COALESCE to default to an empty array if it's nil
	_, err = tx.Exec(ctx,
		`INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties)
			 VALUES ($1, $2, $3, $4, COALESCE($5, '{}'::text[]), $6);`,
		tenantID, mapping.Blocked, mapping.IssuerURL, mapping.JWKSURI, mapping.Audiences, propsBytes,
	)
	if err != nil {
		span.RecordError(err)
		if err, ok := handlePgError(err); ok {
			return err
		}

		return fmt.Errorf("inserting into trust: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID string) error {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "delete_oidc_mapping_sql")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx, `DELETE FROM trust WHERE tenant_id = $1;`, tenantID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("executing sql query: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return serviceerr.ErrNotFound
	}

	err = tx.Commit(ctx)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("committing tx: %w", err)
	}

	return nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, mapping trust.OIDCMapping) error {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "update_oidc_mapping_sql")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	propsBytes, err := r.marshalProperties(mapping)
	if err != nil {
		span.RecordError(err)
		return err
	}

	// The audiences value is optional, so we use COALESCE to default to an empty array if it's nil
	ct, err := tx.Exec(ctx,
		`UPDATE trust
			 SET blocked = $1, issuer = $2, jwks_uri = $3, audiences = COALESCE($4, '{}'::text[]), properties = $5
			 WHERE tenant_id = $6;`,
		mapping.Blocked, mapping.IssuerURL, mapping.JWKSURI, mapping.Audiences, propsBytes, tenantID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("updating trust: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return serviceerr.ErrNotFound
	}

	err = tx.Commit(ctx)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("committing tx: %w", err)
	}

	return nil
}

func (r *Repository) marshalProperties(mapping trust.OIDCMapping) ([]byte, error) {
	propsBytes, err := json.Marshal(mapping.Properties)
	if err != nil {
		return nil, fmt.Errorf("marshaling json: %w", err)
	}
	return propsBytes, nil
}
