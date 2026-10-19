//go:build helmtests

package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
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
			// The line ends with the release-please marker comment.
			value, _, _ = strings.Cut(value, "#")
			return strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	t.Fatal("Chart.yaml has no appVersion")
	return ""
}

// manifest holds the fields of a rendered resource that the tests read.
type manifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
	Spec struct {
		Template struct {
			Metadata struct {
				Annotations map[string]string `yaml:"annotations"`
			} `yaml:"metadata"`
			Spec struct {
				Volumes []volume `yaml:"volumes"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// volume is a volume of a rendered pod template.
type volume struct {
	Name      string `yaml:"name"`
	ConfigMap struct {
		Name string `yaml:"name"`
	} `yaml:"configMap"`
}

// renderManifests runs helm template with the given arguments and returns the rendered
// resources, keyed by "<kind>/<name>".
func renderManifests(t *testing.T, args ...string) map[string]manifest {
	t.Helper()
	args = append([]string{"template", appName, path, "--set", "image.tag=latest"}, args...)
	cmd := exec.CommandContext(t.Context(), "helm", args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm template failed: %v\nOutput: %s%s", err, out.String(), stderr.String())
	}

	manifests := make(map[string]manifest)
	for doc := range strings.SplitSeq(out.String(), "\n---\n") {
		var m manifest
		if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
			t.Fatalf("parsing a rendered resource: %v\n%s", err, doc)
		}
		if m.Kind != "" {
			manifests[m.Kind+"/"+m.Metadata.Name] = m
		}
	}
	return manifests
}

// configMaps returns the configuration ConfigMap of the API server and the housekeeper,
// and the migration job's hook copy.
func configMaps(t *testing.T, manifests map[string]manifest) (configMap, hookConfigMap manifest) {
	t.Helper()
	var found []string
	for key, m := range manifests {
		if m.Kind != "ConfigMap" {
			continue
		}
		found = append(found, key)
		if _, hook := m.Metadata.Annotations["helm.sh/hook"]; hook {
			hookConfigMap = m
		} else {
			configMap = m
		}
	}
	if len(found) != 2 || configMap.Kind == "" || hookConfigMap.Kind == "" {
		t.Fatalf("expected a configuration ConfigMap and a hook ConfigMap, got %v", found)
	}
	return configMap, hookConfigMap
}
