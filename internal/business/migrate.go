package business

import (
	"context"
	"fmt"

	"github.com/XSAM/otelsql"
	"github.com/pressly/goose/v3"
	"github.com/samber/oops"

	// Register pgx driver
	_ "github.com/jackc/pgx/v5/stdlib"

	slogctx "github.com/veqryn/slog-context"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"github.com/openkcm/session-manager/internal/config"
	migrations "github.com/openkcm/session-manager/sql"
)

// MigrateMain starts the database migration
func MigrateMain(ctx context.Context, cfg *config.Config) error {
	const dialect = "pgx"
	dbSystemName := semconv.DBSystemNamePostgreSQL

	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return fmt.Errorf("making connection string from config: %w", err)
	}

	db, err := otelsql.Open(dialect, connStr, otelsql.WithAttributes(dbSystemName))
	if err != nil {
		return oops.In("main").Wrapf(err, "opening DB connection")
	}

	reg, err := otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(dbSystemName))
	if err != nil {
		return fmt.Errorf("registering db stats metrics: %w", err)
	}

	defer func() {
		err = reg.Unregister()
		if err != nil {
			slogctx.Error(ctx, "failed to unregister db stats metrics", "error", err)
		}
	}()

	goose.SetBaseFS(migrations.FS)

	err = goose.SetDialect(dialect)
	if err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	err = goose.UpContext(ctx, db, ".")
	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	return nil
}
