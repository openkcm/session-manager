package sqltrust

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/modules/oidctrust/internal/sql/queries"
	"github.com/openkcm/session-manager/pkg/serviceerr"
)

type Repository struct {
	queries *queries.Queries
}

func NewRepository(db sessionmanager.Database) *Repository {
	return &Repository{
		queries: queries.New(db),
	}
}

func (r *Repository) Get(ctx context.Context, tenantID string) (*trustv1.Trust, error) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "get_trust_sql")
	defer span.End()

	row, err := r.queries.GetTrust(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serviceerr.ErrNotFound
		}

		return nil, err
	}

	trust := trustv1.Trust_builder{
		TenantId: &tenantID,
		Blocked:  &row.Blocked,
		Oidc: oidcv1.OIDC_builder{
			Audiences: row.Audiences,
		}.Build(),
	}.Build()

	if row.Issuer != "" {
		trust.GetOidc().SetIssuer(row.Issuer)
	}

	if row.JwksUri != "" {
		trust.GetOidc().SetJwksUri(row.JwksUri)
	}

	if row.ClientID.Valid {
		trust.GetOidc().SetClientId(row.ClientID.String)
	}

	return trust, nil
}

func (r *Repository) Create(ctx context.Context, trust *trustv1.Trust) error {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "create_trust_sql")
	defer span.End()

	oidc := trust.GetOidc()

	if err := r.queries.CreateTrust(ctx, queries.CreateTrustParams{
		TenantID:  trust.GetTenantId(),
		Blocked:   trust.GetBlocked(),
		Issuer:    oidc.GetIssuer(),
		JwksUri:   oidc.GetJwksUri(),
		Audiences: oidc.GetAudiences(),
		ClientID:  pgTextOrNull(trust.GetOidc().GetClientId()),
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
	ctx, span := tracer.Tracer("").Start(ctx, "delete_trust_sql")
	defer span.End()

	affected, err := r.queries.DeleteTrust(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("executing sql query: %w", err)
	}

	if affected == 0 {
		return serviceerr.ErrNotFound
	}

	return nil
}

func (r *Repository) Update(ctx context.Context, trust *trustv1.Trust) error {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("").Start(ctx, "update_trust_sql")
	defer span.End()

	oidc := trust.GetOidc()

	affected, err := r.queries.UpdateTrust(ctx, queries.UpdateTrustParams{
		Blocked:   trust.GetBlocked(),
		Issuer:    oidc.GetIssuer(),
		JwksUri:   oidc.GetJwksUri(),
		Audiences: oidc.GetAudiences(),
		ClientID:  pgTextOrNull(oidc.GetClientId()),
		TenantID:  trust.GetTenantId(),
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

func pgTextOrNull(s string) pgtype.Text {
	return pgtype.Text{
		String: s,
		Valid:  s != "",
	}
}

func handlePgError(err error) (error, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return serviceerr.ErrConflict, true
	}

	return err, false
}
