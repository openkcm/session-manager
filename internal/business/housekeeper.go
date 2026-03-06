package business

import (
	"context"
	"fmt"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
)

// HousekeeperMain starts the house keeping jobs
func HousekeeperMain(ctx context.Context, cfg *config.Config) error {
	sessionManager, closeFn, err := initSessionManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialise the session manager: %w", err)
	}
	defer closeFn()

	// Start the housekeeper loop
	c := time.Tick(cfg.Housekeeper.TriggerInterval)
	refreshTriggerInterval := cfg.Housekeeper.TokenRefreshTriggerInterval
	concurrencyLimit := cfg.Housekeeper.ConcurrencyLimit
	for {
		err := sessionManager.TriggerHousekeeping(ctx, concurrencyLimit, refreshTriggerInterval)
		if err != nil {
			slogctx.Error(ctx, "Error during session housekeeping", "error", err)
		}

		select {
		case <-c:
			continue
		case <-ctx.Done():
			return nil
		}
	}
}
