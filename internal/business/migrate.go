package business

import (
	"context"
	"fmt"

	// Register pgx driver
	_ "github.com/jackc/pgx/v5/stdlib"

	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
)

// MigrateMain starts the database migration
func MigrateMain(ctx context.Context, cfg *config.Config) error {
	c, cancel := sessionmanager.NewContext(ctx)

	var err error
	defer func() {
		cancel(err)
	}()

	slogctx.Debug(c, "loading db")
	_, err = c.LoadModule(&cfg.Database)
	if err != nil {
		return fmt.Errorf("loading database module: %w", err)
	}

	slogctx.Debug(c, "loading migrate")
	mod, err := c.LoadModule(&cfg.Migrate)
	if err != nil {
		return fmt.Errorf("loading migration module: %w", err)
	}

	//nolint:forcetypeassert
	migrate := mod.(sessionmanager.Migrate)
	slogctx.Debug(c, "executing migration")
	if err := migrate.Migrate(ctx); err != nil {
		return fmt.Errorf("executing migrations: %w", err)
	}

	return nil
}
