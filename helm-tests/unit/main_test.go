//go:build helmtests

package main_test

import (
	"os"
	"strings"
	"testing"
)

const (
	path    = "../../charts/session-manager"
	appName = "session-manager"
)

// This file defines common constants used across all unit tests.
// The path points to the Helm chart directory, and appName is the release name used in tests.

// TestMain renders the chart into the namespace "default", which the tests expect.
func TestMain(m *testing.M) {
	if err := os.Setenv("HELM_NAMESPACE", "default"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// appVersion returns the chart's appVersion, the default image tag.
func appVersion(t *testing.T) string {
	t.Helper()
	chart, err := os.ReadFile(path + "/Chart.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(chart), "\n") {
		if value, ok := strings.CutPrefix(line, "appVersion:"); ok {
			return strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	t.Fatal("Chart.yaml has no appVersion")
	return ""
}
