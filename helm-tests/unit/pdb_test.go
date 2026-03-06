package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestPDBRendering(t *testing.T) {
	tests := []struct {
		name        string
		values      string
		expected    []string
		shouldExist bool
	}{
		{
			name:   "PDB disabled by default",
			values: "",
			expected: []string{
				"kind: PodDisruptionBudget",
			},
			shouldExist: false,
		},
		{
			name:   "PDB enabled with default minAvailable",
			values: "--set pod.disruptionBudget.enabled=true",
			expected: []string{
				"kind: PodDisruptionBudget",
				"minAvailable: 1",
			},
			shouldExist: true,
		},
		{
			name:   "PDB enabled with custom minAvailable",
			values: "--set pod.disruptionBudget.enabled=true --set pod.disruptionBudget.minAvailable=2",
			expected: []string{
				"kind: PodDisruptionBudget",
				"minAvailable: 2",
			},
			shouldExist: true,
		},
		{
			name:   "PDB enabled with maxUnavailable",
			values: "--set pod.disruptionBudget.enabled=true --set pod.disruptionBudget.maxUnavailable=1",
			expected: []string{
				"kind: PodDisruptionBudget",
				"maxUnavailable: 1",
			},
			shouldExist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path, "-s", "templates/session-manager/pdb.yaml"}
			if tt.values != "" {
				args = append(args, strings.Split(tt.values, " ")...)
			}

			cmd := exec.Command("helm", args...)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out

			err := cmd.Run()
			if err != nil && tt.shouldExist {
				t.Fatalf("helm template failed: %v\nOutput: %s", err, out.String())
			}

			if tt.shouldExist {
				output := out.String()
				for _, expected := range tt.expected {
					if !strings.Contains(output, expected) {
						t.Errorf("expected output to contain %q, but it didn't.\nOutput: %s", expected, output)
					}
				}
			}
		})
	}
}
