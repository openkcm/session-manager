package tokenrefresh

import (
	"github.com/spf13/cobra"

	"github.com/openkcm/session-manager/internal/business"
	"github.com/openkcm/session-manager/internal/cmdutils"
)

func Cmd(buildInfo string) *cobra.Command {
	return cmdutils.CobraCommand(
		"token-refresher",
		"Session Manager Token Refresh job",
		"Session Manager Token Refresh job refreshes access tokens",
		buildInfo,
		cmdutils.RunAsService,
		business.TokenRefresherMain,
	)
}
