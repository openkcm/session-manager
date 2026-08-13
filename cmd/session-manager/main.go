package main

import (
	"github.com/openkcm/session-manager/cmd/session-manager/maincmd"
	_ "github.com/openkcm/session-manager/modules/standard"
)

// BuildInfo will be set by the build system
var BuildInfo = "{}"

func main() {
	maincmd.BuildInfo = BuildInfo
	maincmd.Main()
}
