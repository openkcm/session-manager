package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/utils"
	"github.com/pressly/goose/v3"
	"github.com/samber/oops"

	_ "github.com/jackc/pgx/v5/stdlib"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
	migrations "github.com/openkcm/session-manager/sql"
)

var (
	BuildInfo = "{}"

	versionFlag             = flag.Bool("version", false, "print version information")
	gracefulShutdownSec     = flag.Int64("graceful-shutdown", 1, "graceful shutdown seconds")
	gracefulShutdownMessage = flag.String("graceful-shutdown-message", "Graceful shutdown in %d seconds",
		"graceful shutdown message")
)

// run does the heavy lifting until the service is up and running. It will:
//   - Load the config and initializes the logger
//   - Start the status server in a goroutine
//   - Start the business logic and eventually return the error from it
func run(ctx context.Context) error {
	// Load Configuration
	defaultValues := map[string]any{}
	cfg := new(config.Config)

	err := commoncfg.LoadConfig(cfg, defaultValues, "/etc/session-manager", "$HOME/.session-manager", ".")
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to load the configuration")
	}

	err = commoncfg.UpdateConfigVersion(&cfg.BaseConfig, BuildInfo)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to update the version configuration")
	}

	// LoggerConfig initialisation
	err = logger.InitAsDefault(cfg.Logger, cfg.Application)
	if err != nil {
		return oops.In("main").
			Wrapf(err, "Failed to initialise the logger")
	}

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

// runFuncWithSignalHandling runs the given function with signal handling. When
// a CTRL-C is received, the context will be cancelled on which the function can
// act upon.
func runFuncWithSignalHandling(f func(context.Context) error) int {
	ctx, cancelOnSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancelOnSignal()

	exitCode := 0

	if err := f(ctx); err != nil {
		slogctx.Error(ctx, "Failed to start the application", "error", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}

	// graceful shutdown so running goroutines may finish
	_, _ = fmt.Fprintln(os.Stderr, fmt.Sprintf(*gracefulShutdownMessage, *gracefulShutdownSec))
	time.Sleep(time.Duration(*gracefulShutdownSec) * time.Second)

	return exitCode
}

// main is the entry point for the application. It is intentionally kept small
// because it is hard to test, which would lower test coverage.
func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Println(utils.ExtractFromComplexValue(BuildInfo))
		os.Exit(0)
	}

	exitCode := runFuncWithSignalHandling(run)
	os.Exit(exitCode)
}
