package business

import (
	"context"
	"fmt"
	"time"

	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/sessionwiring"
)

// HousekeeperMain starts the house keeping jobs
func HousekeeperMain(ctx context.Context, cfg *config.Config) error {
	c, cancelCause := sessionmanager.NewContext(ctx)
	defer cancelCause(nil)

	c = config.WithContext(c, cfg)

	if err := c.LoadAll([]sessionmanager.LoadSpec{
		{Cfg: &cfg.Database},
		{Cfg: &cfg.Trust},
		{Cfg: &cfg.ValKey},
		{Cfg: &cfg.Credentials},
	}); err != nil {
		return fmt.Errorf("loading shared modules: %w", err)
	}

	trust, err := sessionmanager.GetModuleAs[sessionmanager.Trust](c, cfg.Trust.Module())
	if err != nil {
		return fmt.Errorf("getting trust module: %w", err)
	}

	sessionManager, closeFn, err := sessionwiring.InitSessionManager(c, cfg, trust)
	if err != nil {
		return fmt.Errorf("failed to initialise the session manager: %w", err)
	}
	defer closeFn()

	// Start the housekeeper loop
	ticker := time.NewTicker(cfg.Housekeeper.TriggerInterval)
	defer ticker.Stop()
	refreshTriggerInterval := cfg.Housekeeper.TokenRefreshTriggerInterval
	concurrencyLimit := cfg.Housekeeper.ConcurrencyLimit
	for {
		err := sessionManager.TriggerHousekeeping(c, concurrencyLimit, refreshTriggerInterval)
		if err != nil {
			slogctx.Error(ctx, "Error during session housekeeping", "error", err)
		}

		select {
		case <-ticker.C:
			continue
		case <-c.Done():
			return nil
		}
	}
}
