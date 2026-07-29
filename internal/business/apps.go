package business

import (
	"errors"
	"fmt"
	"slices"

	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
)

// startedApp pairs a configured name with its loaded handle so we can stop
// apps in reverse start order and emit meaningful logs.
type startedApp struct {
	name string
	app  sessionmanager.App
}

// startApps loads, provisions and starts every app declared under the
// top-level apps: section in cfg, in the order given by cfg.AppsOrder (with
// any apps not listed there appended in cfg.Apps map iteration order).
//
// If any Start() returns a non-nil error, every previously-started app is
// stopped in reverse order before the original error is returned. On
// success, the returned stopAll closure stops every started app in reverse
// order and joins any Stop() errors via errors.Join.
func startApps(ctx *sessionmanager.Context, cfg *config.Config) (stopAll func() error, _ error) {
	order, err := appsStartOrder(cfg)
	if err != nil {
		return nil, err
	}

	started := make([]startedApp, 0, len(order))

	rollback := func() {
		for _, sa := range slices.Backward(started) {
			if stopErr := sa.app.Stop(); stopErr != nil {
				slogctx.Error(ctx, "stopping app during rollback", "app", sa.name, "error", stopErr)
			}
		}
	}

	for _, name := range order {
		appCfg := cfg.Apps[name]

		slogctx.Info(ctx, "loading app", "app", name, "module", appCfg.Module())
		app, err := ctx.LoadApp(appCfg)
		if err != nil {
			rollback()
			return nil, fmt.Errorf("loading app %q: %w", name, err)
		}

		slogctx.Info(ctx, "starting app", "app", name)
		if err := app.Start(); err != nil {
			rollback()
			return nil, fmt.Errorf("starting app %q: %w", name, err)
		}

		started = append(started, startedApp{name: name, app: app})
	}

	return func() error {
		var errs []error
		for _, sa := range slices.Backward(started) {
			slogctx.Info(ctx, "stopping app", "app", sa.name)
			if err := sa.app.Stop(); err != nil {
				slogctx.Error(ctx, "stopping app", "app", sa.name, "error", err)
				errs = append(errs, fmt.Errorf("stopping app %q: %w", sa.name, err))
			}
		}
		return errors.Join(errs...)
	}, nil
}

// appsStartOrder returns the names of configured apps in start order. Names
// listed in cfg.AppsOrder come first, in the given order; the remainder are
// appended in cfg.Apps map iteration order. Names in AppsOrder that are not
// present in cfg.Apps surface as a configuration error.
func appsStartOrder(cfg *config.Config) ([]string, error) {
	seen := make(map[string]bool, len(cfg.Apps))
	order := make([]string, 0, len(cfg.Apps))

	for _, name := range cfg.AppsOrder {
		if _, ok := cfg.Apps[name]; !ok {
			return nil, fmt.Errorf("appsOrder references unknown app %q", name)
		}
		if !seen[name] {
			seen[name] = true
			order = append(order, name)
		}
	}

	for name := range cfg.Apps {
		if !seen[name] {
			seen[name] = true
			order = append(order, name)
		}
	}

	return order, nil
}
