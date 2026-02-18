package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestServiceRendering(t *testing.T) {
	tests := []struct {
		name     string
		values   string
		expected []string
	}{
		{
			name:   "default service",
			values: "",
			expected: []string{
				"kind: Service",
				"name: session-manager",
				"app.kubernetes.io/name: session-manager",
				"app.kubernetes.io/component: session-manager",
				"type: ClusterIP",
			},
		},
		{
			name:   "custom service type",
			values: "--set service.type=LoadBalancer",
			expected: []string{
				"type: LoadBalancer",
			},
		},
		{
			name:   "custom service port",
			values: "--set service.ports[0].port=8080",
			expected: []string{
				"port: 8080",
			},
		},
		{
			name:   "default ports configuration",
			values: "",
			expected: []string{
				"ports:",
				"name: http",
				"port: 8080",
				"protocol: TCP",
				"targetPort: http",
				"name: http-status",
				"port: 8888",
				"targetPort: http-status",
				"name: grpc",
				"port: 9091",
				"targetPort: grpc",
			},
		},
		{
			name:   "with service labels",
			values: "--set service.labels.team=backend --set service.labels.stage=prod",
			expected: []string{
				"labels:",
				"stage: prod",
				"team: backend",
			},
		},
		{
			name:   "with service annotations",
			values: "--set service.annotations.prometheus\\.io/scrape=true --set service.annotations.prometheus\\.io/port=8080",
			expected: []string{
				"annotations:",
				"prometheus.io/port: 8080",
				"prometheus.io/scrape: true",
			},
		},
		{
			name:   "with selector labels",
			values: "",
			expected: []string{
				"selector:",
				"app.kubernetes.io/name: session-manager",
				"app.kubernetes.io/instance: session-manager",
				"app.kubernetes.io/component: session-manager",
			},
		},
		{
			name:   "with NodePort service type",
			values: "--set service.type=NodePort",
			expected: []string{
				"type: NodePort",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path, "-s", "templates/session-manager/service.yaml"}
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
					t.Errorf("expected output to contain %q, but it didn't.\nOutput: %s", expected, output)
				}
			}
		})
	}
}
