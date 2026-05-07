package business

import (
	"context"
	"fmt"
	"time"

	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
)

// HousekeeperMain starts the house keeping jobs
func HousekeeperMain(ctx context.Context, cfg *config.Config) error {
	c, cancelCause := sessionmanager.NewContext(ctx)
	defer cancelCause(nil)

	_, err := c.LoadModule(&cfg.Database)
	if err != nil {
		return fmt.Errorf("loading database module: %w", err)
	}

	trustMod, err := c.LoadModule(&cfg.Trust)
	if err != nil {
		return fmt.Errorf("loading trust module: %w", err)
	}

	//nolint:forcetypeassert
	trust := trustMod.(sessionmanager.Trust)

	sessionManager, closeFn, err := initSessionManager(ctx, cfg, trust)
	if err != nil {
		return fmt.Errorf("failed to initialise the session manager: %w", err)
	}
	defer closeFn()

	// Start the housekeeper loop
	tick := time.Tick(cfg.Housekeeper.TriggerInterval)
	refreshTriggerInterval := cfg.Housekeeper.TokenRefreshTriggerInterval
	concurrencyLimit := cfg.Housekeeper.ConcurrencyLimit
	for {
		err := sessionManager.TriggerHousekeeping(c, concurrencyLimit, refreshTriggerInterval)
		if err != nil {
			slogctx.Error(ctx, "Error during session housekeeping", "error", err)
		}

		select {
		case <-tick:
			continue
		case <-c.Done():
			return nil
		}
	}
}
