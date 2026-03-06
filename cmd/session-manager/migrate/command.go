package migrate

import (
	"github.com/spf13/cobra"

	"github.com/openkcm/session-manager/internal/business"
	"github.com/openkcm/session-manager/internal/cmdutils"
)

func Cmd(buildInfo string) *cobra.Command {
	return cmdutils.CobraCommand(
		"migrate",
		"Session Manager database migrations",
		"Session Manager database migrations keeps the database up to date.",
		buildInfo,
		cmdutils.RunAsJob,
		business.MigrateMain,
	)
}
