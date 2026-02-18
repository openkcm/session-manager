package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestHPARendering(t *testing.T) {
	tests := []struct {
		name        string
		values      string
		expected    []string
		shouldExist bool
	}{
		{
			name:   "HPA disabled by default",
			values: "",
			expected: []string{
				"kind: HorizontalPodAutoscaler",
			},
			shouldExist: false,
		},
		{
			name:   "HPA enabled with default replicas",
			values: "--set autoscaling.enabled=true",
			expected: []string{
				"kind: HorizontalPodAutoscaler",
				"minReplicas: 1",
				"maxReplicas: 1",
				"averageUtilization: 80",
			},
			shouldExist: true,
		},
		{
			name:   "HPA enabled with custom replicas",
			values: "--set autoscaling.enabled=true --set autoscaling.minReplicas=2 --set autoscaling.maxReplicas=10",
			expected: []string{
				"kind: HorizontalPodAutoscaler",
				"minReplicas: 2",
				"maxReplicas: 10",
			},
			shouldExist: true,
		},
		{
			name:   "HPA with custom CPU and memory targets",
			values: "--set autoscaling.enabled=true --set autoscaling.minReplicas=1 --set autoscaling.maxReplicas=5 --set autoscaling.targetCPUUtilizationPercentage=70 --set autoscaling.targetMemoryUtilizationPercentage=85",
			expected: []string{
				"kind: HorizontalPodAutoscaler",
				"minReplicas: 1",
				"maxReplicas: 5",
				"name: cpu",
				"averageUtilization: 70",
				"name: memory",
				"averageUtilization: 85",
			},
			shouldExist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path, "-s", "templates/session-manager/hpa.yaml"}
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
