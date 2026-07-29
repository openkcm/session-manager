package business

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/business/server"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/sessionwiring"
)

// Main starts the public HTTP API server and the configured apps.
func Main(ctx context.Context, cfg *config.Config) error {
	c, cancelCause := sessionmanager.NewContext(ctx)
	defer cancelCause(nil)

	c = config.WithContext(c, cfg)

	if _, err := c.LoadModule(&cfg.Database); err != nil {
		return fmt.Errorf("loading database module: %w", err)
	}

	if _, err := c.LoadModule(&cfg.Trust); err != nil {
		return fmt.Errorf("loading trust module: %w", err)
	}

	if _, err := c.LoadModule(&cfg.ValKey); err != nil {
		return fmt.Errorf("loading session-store module: %w", err)
	}

	if _, err := c.LoadModule(&cfg.Credentials); err != nil {
		return fmt.Errorf("loading credentials module: %w", err)
	}

	stopApps, err := startApps(c, cfg)
	if err != nil {
		return fmt.Errorf("starting apps: %w", err)
	}

	// errChan captures the first error and triggers shutdown.
	errChan := make(chan error, 1)

	// wg is used to wait for all goroutines to shutdown.
	var wg sync.WaitGroup

	// start public HTTP REST API server
	wg.Go(func() {
		errChan <- publicMain(c, cfg)
	})

	// wait for any error to initiate the shutdown
	err = <-errChan
	if err != nil {
		slogctx.Error(ctx, "Shutting down servers", "error", err)
	}

	stopErr := stopApps()
	cancelCause(err)

	wg.Wait()

	return errors.Join(err, stopErr)
}

// publicMain starts the HTTP REST public API server.
func publicMain(ctx *sessionmanager.Context, cfg *config.Config) error {
	csrfSecret, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.CSRFSecret)
	if err != nil {
		return fmt.Errorf("loading csrf token from source ref: %w", err)
	}
	if len(csrfSecret) < 32 {
		return errors.New("CSRF secret must be at least 32 bytes")
	}

	cfg.SessionManager.CSRFSecretParsed = csrfSecret

	trustMod, err := ctx.GetModule(cfg.Trust.Module())
	if err != nil {
		return fmt.Errorf("getting trust module: %w", err)
	}

	//nolint:forcetypeassert
	trust := trustMod.(sessionmanager.Trust)

	sessionManager, closeFn, err := sessionwiring.InitSessionManager(ctx, cfg, trust)
	if err != nil {
		return fmt.Errorf("failed to initialise the session manager: %w", err)
	}

	defer closeFn()

	return server.StartHTTPServer(ctx, cfg, sessionManager)
}
