package housekeeper

import (
	"github.com/spf13/cobra"

	"github.com/openkcm/session-manager/internal/business"
	"github.com/openkcm/session-manager/internal/cmdutils"
)

func Cmd(buildInfo string) *cobra.Command {
	return cmdutils.CobraCommand(
		"housekeeper",
		"Session Manager Housekeeping job",
		"Session Manager Housekeeping job refreshes access tokens, cleanups idle sessions, etc.",
		buildInfo,
		cmdutils.RunAsService,
		business.HousekeeperMain,
	)
}
