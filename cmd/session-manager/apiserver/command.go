package apiserver

import (
	"github.com/spf13/cobra"

	"github.com/openkcm/session-manager/internal/business"
	"github.com/openkcm/session-manager/internal/cmdutils"
)

func Cmd(buildInfo string) *cobra.Command {
	return cmdutils.CobraCommand(
		"api-server",
		"Session Manager API server",
		"Session Manager API server hosts a public http API and a private gRPC API",
		buildInfo,
		cmdutils.RunAsService,
		business.Main,
	)
}
