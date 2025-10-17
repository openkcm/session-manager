package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/openkcm/common-sdk/pkg/utils"
	"github.com/spf13/cobra"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/cmd/session-manager/apiserver"
	"github.com/openkcm/session-manager/cmd/session-manager/migrate"
	tokenrefresh "github.com/openkcm/session-manager/cmd/session-manager/token-refresher"
)

var (
	// BuildInfo will be set by the build system
	BuildInfo = "{}"

	isVersionCmd     bool
	gracefulShutdown time.Duration
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "CMK Version",
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
		Use:   "cmk",
		Short: "OpenKCM CMK - Customer Manager Keys",
		Long: "OpenKCM Customer Manager Keys(CMK) is a key management service to manage " +
			"encryption keys for applications and services.",
	}

	cmd.PersistentFlags().DurationVar(&gracefulShutdown, "graceful-shutdown", 1*time.Second, "graceful shutdown")

	cmd.AddCommand(
		versionCmd,
		apiserver.Cmd(BuildInfo),
		tokenrefresh.Cmd(BuildInfo),
		migrate.Cmd(BuildInfo),
	)

	return cmd
}
func execute() error {
	ctx, cancelOnSignal := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelOnSignal()

	if err := rootCmd().ExecuteContext(ctx); err != nil {
		slogctx.Error(ctx, "failed to start the application", "error", err)
		_, _ = fmt.Fprintln(os.Stderr, err)

		return err
	}

	if !isVersionCmd {
		_, _ = fmt.Fprintf(os.Stderr, "Graceful shutdown in %s\n", gracefulShutdown)
		time.Sleep(gracefulShutdown)
	}

	return nil
}

func main() {
	if err := execute(); err != nil {
		os.Exit(1)
	}
}
