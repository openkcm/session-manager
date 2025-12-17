package business

import (
	"context"
	"fmt"
	"sync"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/session"
)

// HousekeeperMain starts the house keeping jobs
func HousekeeperMain(ctx context.Context, cfg *config.Config) error {
	sessionManager, closeFn, err := initSessionManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialise the session manager: %w", err)
	}
	defer closeFn()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// errChan is used to capture the first error and shutdown the servers.
	errChan := make(chan error, 1)

	// wg is used to wait for all servers to shutdown.
	var wg sync.WaitGroup

	// start expiring token refresh
	wg.Go(func() {
		slogctx.Info(ctx, "Starting expiring token refresh job")
		errChan <- startExpiringTokenRefresh(ctx, sessionManager, &cfg.Housekeeper)
	})

	// start idle session cleanup
	wg.Go(func() {
		slogctx.Info(ctx, "Starting idle session cleanup job")
		errChan <- startIdleSessionCleanup(ctx, sessionManager, &cfg.Housekeeper)
	})

	// wait for any error to initiate the shutdown
	err = <-errChan
	if err != nil {
		slogctx.Error(ctx, "Shutting down servers", "error", err)
	}
	cancel()

	// wait for all servers to shutdown
	wg.Wait()

	return nil
}

func startExpiringTokenRefresh(ctx context.Context, sessionManager *session.Manager, cfg *config.Housekeeper) error {
	c := time.Tick(cfg.TokenRefreshInterval)
	triggerInterval := cfg.TokenRefreshTriggerInterval
	for {
		slogctx.Info(ctx, "Triggering refresh of expiring tokens")
		err := sessionManager.RefreshExpiringTokens(ctx, triggerInterval)
		if err != nil {
			slogctx.Error(ctx, "failed to refresh expiring tokens", "error", err)
		}

		select {
		case <-c:
			continue
		case <-ctx.Done():
			return nil
		}
	}
}

func startIdleSessionCleanup(ctx context.Context, sessionManager *session.Manager, cfg *config.Housekeeper) error {
	c := time.Tick(cfg.IdleSessionCleanupInterval)
	for {
		slogctx.Info(ctx, "Triggering cleanup of idle sessions")
		err := sessionManager.CleanupIdleSessions(ctx, cfg.IdleSessionTimeout)
		if err != nil {
			slogctx.Error(ctx, "failed to cleanup idle sessions", "error", err)
		}

		select {
		case <-c:
			continue
		case <-ctx.Done():
			return nil
		}
	}
}
