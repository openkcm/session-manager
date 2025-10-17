package apiserver

import (
	"context"
	"fmt"
	"log/slog"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/otlp"
	"github.com/openkcm/common-sdk/pkg/status"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/business"
	"github.com/openkcm/session-manager/internal/config"
)

const (
	healthStatusTimeout = 5 * time.Second
)

func run(ctx context.Context, cfg *config.Config) error {
	// LoggerConfig initialisation
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to initialise the logger")
	}

	slogctx.Debug(ctx, "Starting the application", slog.Any("config", cfg))

	// OpenTelemetry initialisation
	err = otlp.Init(ctx, &cfg.Application, &cfg.Telemetry, &cfg.Logger)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to load the telemetry")
	}

	go func() {
		if err := startStatusServer(ctx, cfg); err != nil {
			slogctx.Error(ctx, "Failure on the status server", "error", err)
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}
	}()

	// Business Logic
	err = business.Main(ctx, cfg)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to start the main business application")
	}

	return nil
}

func startStatusServer(ctx context.Context, cfg *config.Config) error {
	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return fmt.Errorf("making connection string from config: %w", err)
	}

	liveness := status.WithLiveness(
		health.NewHandler(
			health.NewChecker(health.WithDisabledAutostart()),
		),
	)

	healthOptions := []health.Option{
		health.WithDisabledAutostart(),
		health.WithTimeout(healthStatusTimeout),
		health.WithDatabaseChecker("pgx", connStr),
		health.WithStatusListener(func(ctx context.Context, state health.State) {
			slogctx.Info(ctx, "readiness status changed", "status", state.Status)
		}),
	}

	readiness := status.WithReadiness(
		health.NewHandler(
			health.NewChecker(healthOptions...),
		),
	)

	if err := status.Start(ctx, &cfg.BaseConfig, liveness, readiness); err != nil {
		return fmt.Errorf("starting status server: %w", err)
	}

	return nil
}

func loadConfig(buildInfo string) (*config.Config, error) {
	defaultValues := map[string]any{}
	cfg := &config.Config{}

	if err := commoncfg.LoadConfig(
		cfg,
		defaultValues,
		"/etc/session-manager",
		"$HOME/.session-manager",
		".",
	); err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}

	// Update Version
	if err := commoncfg.UpdateConfigVersion(
		&cfg.BaseConfig,
		buildInfo,
	); err != nil {
		return nil, fmt.Errorf("updating the version configuration: %w", err)
	}

	return cfg, nil
}

func Cmd(buildInfo string) *cobra.Command {
	return &cobra.Command{
		Use:   "api-server",
		Short: "Session Manager API server",
		Long:  "Session Manager API server hosts a public http API and a private gRPC API",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(buildInfo)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if err := run(cmd.Context(), cfg); err != nil {
				return fmt.Errorf("running the api server: %w", err)
			}

			return nil
		},
	}
}
