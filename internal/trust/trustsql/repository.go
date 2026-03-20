package trustsql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/trust"
	"github.com/openkcm/session-manager/internal/trust/trustsql/internal/queries"
)

type Repository struct {
	db      *pgxpool.Pool
	queries *queries.Queries
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:      db,
		queries: queries.New(db),
	}
}

func (r *Repository) Get(ctx context.Context, tenantID string) (trust.OIDCMapping, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "get_oidc_mapping_sql")
	defer span.End()

	row, err := r.queries.GetOIDCMapping(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, pgx.ErrNoRows) {
			return trust.OIDCMapping{}, serviceerr.ErrNotFound
		}

		return trust.OIDCMapping{}, err
	}

	properties := make(map[string]string)
	if len(row.Properties) > 0 {
		err := json.Unmarshal(row.Properties, &properties)
		if err != nil {
			return trust.OIDCMapping{}, fmt.Errorf("unmarshalling properties: %w", err)
		}
	}

	return trust.OIDCMapping{
		IssuerURL:  row.Issuer,
		Blocked:    row.Blocked,
		JWKSURI:    row.JwksUri,
		Audiences:  row.Audiences,
		Properties: properties,
		ClientID:   row.ClientID.String,
	}, nil
}

func (r *Repository) Create(ctx context.Context, tenantID string, mapping trust.OIDCMapping) error {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "create_oidc_mapping_sql")
	defer span.End()

	properties, err := r.marshalProperties(mapping)
	if err != nil {
		return fmt.Errorf("marshaling properties: %w", err)
	}

	if err := r.queries.CreateOIDCMapping(ctx, queries.CreateOIDCMappingParams{
		TenantID:   tenantID,
		Blocked:    mapping.Blocked,
		Issuer:     mapping.IssuerURL,
		JwksUri:    mapping.JWKSURI,
		Audiences:  mapping.Audiences,
		Properties: properties,
		ClientID:   pgTextOrNull(mapping.ClientID),
	}); err != nil {
		span.RecordError(err)
		if err, ok := handlePgError(err); ok {
			return err
		}

		return fmt.Errorf("inserting into trust: %w", err)
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID string) error {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "delete_oidc_mapping_sql")
	defer span.End()

	affected, err := r.queries.DeleteOIDCMapping(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("executing sql query: %w", err)
	}

	if affected == 0 {
		return serviceerr.ErrNotFound
	}

	return nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, mapping trust.OIDCMapping) error {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "update_oidc_mapping_sql")
	defer span.End()

	properties, err := r.marshalProperties(mapping)
	if err != nil {
		span.RecordError(err)
		return err
	}

	affected, err := r.queries.UpdateOIDCMapping(ctx, queries.UpdateOIDCMappingParams{
		Blocked:    mapping.Blocked,
		Issuer:     mapping.IssuerURL,
		JwksUri:    mapping.JWKSURI,
		Audiences:  mapping.Audiences,
		Properties: properties,
		ClientID:   pgTextOrNull(mapping.ClientID),
		TenantID:   tenantID,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("updating trust: %w", err)
	}

	if affected == 0 {
		return serviceerr.ErrNotFound
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

func pgTextOrNull(s string) pgtype.Text {
	return pgtype.Text{
		String: s,
		Valid:  s != "",
	}
}
