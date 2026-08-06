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

	slogctx.Debug(c, "loading db and migrate modules")
	if err = c.LoadAll([]sessionmanager.LoadSpec{
		{Cfg: &cfg.Database},
		{Cfg: &cfg.Migrate},
	}); err != nil {
		return fmt.Errorf("loading migration modules: %w", err)
	}

	migrate, err := sessionmanager.GetModuleAs[sessionmanager.Migrate](c, cfg.Migrate.Module())
	if err != nil {
		return fmt.Errorf("getting migration module: %w", err)
	}

	slogctx.Debug(c, "executing migration")
	if err := migrate.Migrate(ctx); err != nil {
		return fmt.Errorf("executing migrations: %w", err)
	}

	return nil
}
