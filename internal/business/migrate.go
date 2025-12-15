package business

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/samber/oops"

	// Register pgx driver
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/openkcm/session-manager/internal/config"
	migrations "github.com/openkcm/session-manager/sql"
)

// MigrateMain starts the database migration
func MigrateMain(ctx context.Context, cfg *config.Config) error {
	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return fmt.Errorf("making connection string from config: %w", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return oops.In("main").Wrapf(err, "opening DB connection")
	}

	goose.SetBaseFS(migrations.FS)

	err = goose.SetDialect("pgx")
	if err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	err = goose.UpContext(ctx, db, ".")
	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	return nil
}
