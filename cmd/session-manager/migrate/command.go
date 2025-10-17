package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/pressly/goose/v3"
	"github.com/samber/oops"
	"github.com/spf13/cobra"

	_ "github.com/jackc/pgx/v5/stdlib"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
	migrations "github.com/openkcm/session-manager/sql"
)

func run(ctx context.Context, cfg *config.Config) error {
	// LoggerConfig initialisation
	err := logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to initialise the logger")
	}

	slogctx.Debug(ctx, "Starting the application", slog.Any("config", cfg))

	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return fmt.Errorf("making connection string from config: %w", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return oops.In("main").Wrapf(err, "opening DB connection")
	}

	goose.SetBaseFS(migrations.FS)

	if err := goose.SetDialect("pgx"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
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
		Use:   "migrate",
		Short: "Session Manager migrations",
		Long:  "",
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
