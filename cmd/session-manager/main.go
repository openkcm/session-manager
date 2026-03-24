package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/openkcm/common-sdk/pkg/utils"
	"github.com/spf13/cobra"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/cmd/session-manager/apiserver"
	"github.com/openkcm/session-manager/cmd/session-manager/housekeeper"
	"github.com/openkcm/session-manager/cmd/session-manager/migrate"
)

var (
	// BuildInfo will be set by the build system
	BuildInfo = "{}"

	isVersionCmd     bool
	gracefulShutdown time.Duration
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Session Manager Version",
	RunE: func(cmd *cobra.Command, _ []string) error {
		isVersionCmd = true

		value, err := utils.ExtractFromComplexValue(BuildInfo)
		if err != nil {
			return err
		}

		slog.InfoContext(cmd.Context(), value)

		return nil
	},
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session-manager",
		Short: "Session Manager",
		Long:  "KCM Session Manager, implementing the OIDC authorization code flow.",
	}

	cmd.PersistentFlags().DurationVar(&gracefulShutdown, "graceful-shutdown", 1*time.Second, "graceful shutdown")

	cmd.AddCommand(
		versionCmd,
		apiserver.Cmd(BuildInfo),
		housekeeper.Cmd(BuildInfo),
		migrate.Cmd(BuildInfo),
	)

	return cmd
}

func execute() error {
	ctx, cancelOnSignal := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelOnSignal()

	err := rootCmd().ExecuteContext(ctx)
	if err != nil {
		slogctx.Error(ctx, "failed to start the application", "error", err)
		return err
	}

	if !isVersionCmd {
		slogctx.Info(ctx, "Graceful shutdown", "duration", gracefulShutdown)
		time.Sleep(gracefulShutdown)
	}

	return nil
}

func main() {
	err := execute()
	if err != nil {
		os.Exit(1)
	}
}
