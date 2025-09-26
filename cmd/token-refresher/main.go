package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/utils"
	"github.com/samber/oops"

	"github.com/openkcm/session-manager/internal/business"
	"github.com/openkcm/session-manager/internal/config"
)

var (
	BuildInfo = "{}"

	versionFlag = flag.Bool("version", false, "print version information")
)

func run(ctx context.Context) error {
	flag.Parse()
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

	if err := business.TokenRefresherMain(ctx, cfg); err != nil {
		os.Exit(1)
	}

	return nil
}

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Println(utils.ExtractFromComplexValue(BuildInfo))
		os.Exit(0)
	}

	err := run(context.Background())
	if err != nil {
		return
	}
}
