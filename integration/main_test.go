//go:build integration

package integration_test

import (
	"context"
	"log"
	"os"
	"os/exec"
	"testing"
)

var binaries = []string{"session-manager"}

var validConfig string

func init() {
	// read config file
	dat, err := os.ReadFile("../config.yaml")
	if err != nil {
		panic(err)
	}

	validConfig = string(dat)
}

func buildCommandsAndRunTests(m *testing.M, cmds ...string) int {
	for _, name := range cmds {
		cmd := exec.CommandContext(context.Background(), "go", "build", "-buildvcs=false", "-race", "-cover", "-o", name, "../cmd/"+name)
		output, err := cmd.CombinedOutput()
		if err != nil {
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
