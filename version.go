package root

import (
	_ "embed"
)

// generated in https://github.com/openkcm/build/tree/main/.github/workflows
//
//go:embed build_version.json
var BuildVersion string
