//go:build helmtests

package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestConfigMapRendering(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name       string
		values     string
		expected   []string
		unexpected []string
	}{
		{
			name:   "default configmap",
			values: "",
			expected: []string{
				"kind: ConfigMap",
				"name: session-manager\n",
				"namespace: default",
				"application:",
				"name: session-manager",
				`environment: "production"`,
				"reflection: false",
			},
			// The ConfigMap is a regular release resource, not a Helm hook. Secrets have no
			// default values.
			unexpected: []string{
				"helm.sh/hook",
				"immutable:",
				"session-manager-config",
				"my-csrf-secret",
			},
		},
		{
			name:   "immutable configmap",
			values: "--set config.isImmutable=true",
			expected: []string{
				"kind: ConfigMap",
				"immutable: true",
			},
			// Chart-only keys are not written to config.yaml.
			unexpected: []string{
				"isImmutable",
			},
		},
		{
			name:   "immutable configmap with configMap.immutable",
			values: "--set configMap.immutable=true",
			expected: []string{
				"immutable: true",
				"name: session-manager-",
			},
		},
		{
			name:   "optional configuration sections are written when set",
			values: "--set config.audit.endpoint=http://collector:4318/v1/logs --set config.appsOrder[0]=grpc --set config.sessionManager.allowedRedirectBaseURLs[0]=https://ui.example.com/",
			expected: []string{
				"audit:",
				"endpoint: http://collector:4318/v1/logs",
				"appsOrder:",
				"- https://ui.example.com/",
			},
		},
		{
			// templates/configmap.yaml lists the Session Manager settings; other keys are
			// not written.
			name:   "keys that are not Session Manager settings are not written",
			values: "--set config.featureFlags.enabled=true --set config.unknown=x",
			unexpected: []string{
				"featureFlags",
				"unknown",
			},
		},
		{
			name:   "application block is built from the chart values",
			values: "--set config.environment=staging --set config.application.name=sm-eu10",
			expected: []string{
				"application:",
				`environment: "staging"`,
				"name: sm-eu10",
			},
			// The flat chart keys are not written at the top level.
			unexpected: []string{
				"\n    environment:",
				"\n    labels:",
			},
		},
		{
			name:   "custom environment",
			values: "--set config.environment=development",
			expected: []string{
				`environment: "development"`,
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
			values: "--set config.migrate.dbModule=database.module.custom",
			expected: []string{
				"migrate:",
				"dbModule: database.module.custom",
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
			args := []string{"template", appName, path, "-s", "templates/configmap.yaml",
				"--set", "image.tag=latest",
			}
			if tt.values != "" {
				args = append(args, strings.Split(tt.values, " ")...)
			}

			cmd := exec.CommandContext(ctx, "helm", args...)
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
			for _, unexpected := range tt.unexpected {
				if strings.Contains(output, unexpected) {
					t.Errorf("expected output NOT to contain %q, but it did.\nOutput: %s", unexpected, output)
				}
			}
		})
	}
}
