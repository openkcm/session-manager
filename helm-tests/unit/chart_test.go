package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestFullChartRendering(t *testing.T) {
	tests := []struct {
		name             string
		values           string
		expectedCount    map[string]int
		shouldNotContain []string
	}{
		{
			name:   "default values",
			values: "",
			expectedCount: map[string]int{
				"kind: Deployment":     2, // session-manager and housekeeper
				"kind: Service":        1,
				"kind: ServiceAccount": 1,
				"kind: Role":           1,
				"kind: RoleBinding":    1,
				"kind: ConfigMap":      1,
				"kind: Job":            1, // migrate job
			},
			shouldNotContain: []string{
				"kind: HorizontalPodAutoscaler", // HPA disabled by default
				"kind: PodDisruptionBudget",     // PDB disabled by default
			},
		},
		{
			name:   "with name override",
			values: "--set nameOverride=custom-name",
			expectedCount: map[string]int{
				"app.kubernetes.io/name: custom-name": 1,
			},
		},
		{
			name:   "with multiple replicas",
			values: "--set replicaCount=5",
			expectedCount: map[string]int{
				"replicas: 5": 1,
			},
		},
		{
			name:   "with HPA enabled",
			values: "--set autoscaling.enabled=true",
			expectedCount: map[string]int{
				"kind: HorizontalPodAutoscaler": 1,
			},
		},
		{
			name:   "with PDB enabled",
			values: "--set pod.disruptionBudget.enabled=true",
			expectedCount: map[string]int{
				"kind: PodDisruptionBudget": 1,
			},
		},
		{
			name:   "with ServiceAccount disabled",
			values: "--set serviceAccount.create=false",
			expectedCount: map[string]int{
				"kind: Deployment": 2, // Still have deployments
			},
			shouldNotContain: []string{
				"kind: ServiceAccount",
				"kind: Role",
				"kind: RoleBinding",
			},
		},
		{
			name:   "with all optional resources enabled",
			values: "--set autoscaling.enabled=true --set pod.disruptionBudget.enabled=true",
			expectedCount: map[string]int{
				"kind: Deployment":              2,
				"kind: Service":                 1,
				"kind: ServiceAccount":          1,
				"kind: Role":                    1,
				"kind: RoleBinding":             1,
				"kind: ConfigMap":               1,
				"kind: Job":                     1,
				"kind: HorizontalPodAutoscaler": 1,
				"kind: PodDisruptionBudget":     1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path}
			if tt.values != "" {
				args = append(args, strings.Split(tt.values, " ")...)
			}

			cmd := exec.Command("helm", args...)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out

			err := cmd.Run()
			if err != nil {
				t.Fatalf("helm template failed: %v\nOutput: %s", err, out.String())
			}

			output := out.String()

			// Check expected counts
			for expected, count := range tt.expectedCount {
				actual := strings.Count(output, expected)
				if actual < count {
					t.Errorf("expected at least %d occurrences of %q, got %d", count, expected, actual)
				}
			}

			// Check strings that should not be present
			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(output, notExpected) {
					t.Errorf("expected output to NOT contain %q, but it did", notExpected)
				}
			}
		})
	}
}

func TestChartValidation(t *testing.T) {
	tests := []struct {
		name        string
		values      string
		shouldFail  bool
		errorString string
	}{
		{
			name:       "valid default chart",
			values:     "",
			shouldFail: false,
		},
		{
			name:       "valid with custom values",
			values:     "--set replicaCount=3 --set image.tag=v1.0.0",
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path, "--debug"}
			if tt.values != "" {
				args = append(args, strings.Split(tt.values, " ")...)
			}

			cmd := exec.Command("helm", args...)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out

			err := cmd.Run()

			if tt.shouldFail && err == nil {
				t.Errorf("expected helm template to fail, but it succeeded")
			}

			if !tt.shouldFail && err != nil {
				t.Errorf("expected helm template to succeed, but it failed: %v\nOutput: %s", err, out.String())
			}

			if tt.shouldFail && tt.errorString != "" {
				output := out.String()
				if !strings.Contains(output, tt.errorString) {
					t.Errorf("expected error to contain %q, but got: %s", tt.errorString, output)
				}
			}
		})
	}
}

func TestLabelsAndAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		values   string
		expected []string
	}{
		{
			name:   "common labels present",
			values: "",
			expected: []string{
				"app.kubernetes.io/name: session-manager",
				"app.kubernetes.io/instance:",
				"app.kubernetes.io/managed-by: Helm",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path}
			if tt.values != "" {
				args = append(args, strings.Split(tt.values, " ")...)
			}

			cmd := exec.Command("helm", args...)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out

			err := cmd.Run()
			if err != nil {
				t.Fatalf("helm template failed: %v\nOutput: %s", err, out.String())
			}

			output := out.String()
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q, but it didn't", expected)
				}
			}
		})
	}
}
