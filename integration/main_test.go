//go:build integration

package integration_test

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
)

var binaries = []string{"api-pub", "api-int", "migrate"}

var validConfig, buildVersion string

func init() {
	// read config file
	dat, err := os.ReadFile("../config.yaml")
	if err != nil {
		panic(err)
	}
	validConfig = string(dat)

	// read build_version.json file
	dat, err = os.ReadFile("../build_version.json")
	if err != nil {
		panic(err)
	}
	buildVersion = strings.TrimSpace(string(dat))
}

func buildCommandsAndRunTests(m *testing.M, cmds ...string) int {
	for _, name := range cmds {
		cmd := exec.Command("go", "build", "-buildvcs=false", "-race", "-cover", "-o", name, "../cmd/"+name)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("output: %s", output)
			log.Fatalf("error: %v", err)
		}
		defer os.Remove(name)
	}

	code := m.Run()
	return code
}

func TestMain(m *testing.M) {
	code := buildCommandsAndRunTests(m, binaries...)
	os.Exit(code)
}
