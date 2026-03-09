package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestConfigMapRendering(t *testing.T) {
	tests := []struct {
		name     string
		values   string
		expected []string
	}{
		{
			name:   "default configmap",
			values: "",
			expected: []string{
				"kind: ConfigMap",
				"name: session-manager-config",
				"immutable: false",
				"helm.sh/hook: pre-install,pre-upgrade",
				"helm.sh/weight: \"-1\"",
				"helm.sh/hook-delete-policy: before-hook-creation",
				"application:",
				"name: session-manager",
				"environment: production",
			},
		},
		{
			name:   "immutable configmap",
			values: "--set config.isImmutable=true",
			expected: []string{
				"kind: ConfigMap",
				"immutable: true",
			},
		},
		{
			name:   "custom environment",
			values: "--set config.environment=development",
			expected: []string{
				"environment: development",
			},
		},
		{
			name:   "custom database config",
			values: "--set config.database.host.value=custom-db.example.com --set config.database.port=5433",
			expected: []string{
				"database:",
				"value: custom-db.example.com",
				"port: 5433",
			},
		},
		{
			name:   "custom valkey config",
			values: "--set config.valkey.host.value=custom-valkey.example.com --set config.valkey.prefix=custom-prefix",
			expected: []string{
				"valkey:",
				"value: custom-valkey.example.com",
				"prefix: custom-prefix",
			},
		},
		{
			name:   "custom sessionManager config",
			values: "--set config.sessionManager.sessionDuration=24h --set config.sessionManager.idleSessionTimeout=2h",
			expected: []string{
				"sessionManager:",
				"sessionDuration: 24h",
				"idleSessionTimeout: 2h",
			},
		},
		{
			name:   "custom housekeeper config",
			values: "--set config.housekeeper.triggerInterval=5m --set config.housekeeper.concurrencyLimit=20",
			expected: []string{
				"housekeeper:",
				"triggerInterval: 5m",
				"concurrencyLimit: 20",
			},
		},
		{
			name:   "custom logger config",
			values: "--set config.logger.level=debug --set config.logger.format=text",
			expected: []string{
				"logger:",
				"level: debug",
				"format: text",
			},
		},
		{
			name:   "custom audit config",
			values: "--set config.audit.endpoint=http://audit-server:8080/logs",
			expected: []string{
				"audit:",
				"endpoint: http://audit-server:8080/logs",
			},
		},
		{
			name:   "custom http config",
			values: "--set config.http.address=:9090",
			expected: []string{
				"http:",
				"address: :9090",
			},
		},
		{
			name:   "custom grpc config",
			values: "--set config.grpc.address=:9092 --set config.grpc.flags.reflection=false",
			expected: []string{
				"grpc:",
				"address: :9092",
				"reflection: false",
			},
		},
		{
			name:   "custom status config",
			values: "--set config.status.enabled=false --set config.status.profiling=true",
			expected: []string{
				"status:",
				"enabled: false",
				"profiling: true",
			},
		},
		{
			name:   "custom telemetry config",
			values: "--set config.telemetry.logs.enabled=true --set config.telemetry.traces.enabled=true",
			expected: []string{
				"telemetry:",
				"logs:",
				"enabled: true",
				"traces:",
				"enabled: true",
			},
		},
		{
			name:   "custom migrate config",
			values: "--set config.migrate.source=file:///custom-path",
			expected: []string{
				"migrate:",
				"source: file:///custom-path",
			},
		},
		{
			name:   "custom application labels",
			values: "--set config.labels.team=backend --set config.labels.stage=staging",
			expected: []string{
				"labels:",
				"stage: staging",
				"team: backend",
			},
		},
		{
			name:   "custom tokenRefreshTriggerInterval",
			values: "--set config.housekeeper.tokenRefreshTriggerInterval=10m",
			expected: []string{
				"housekeeper:",
				"tokenRefreshTriggerInterval: 10m",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path, "-s", "templates/configmap.yaml"}
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
