package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestMigrateJobRendering(t *testing.T) {
	tests := []struct {
		name     string
		values   string
		expected []string
	}{
		{
			name:   "default migrate job",
			values: "",
			expected: []string{
				"kind: Job",
				"name: session-manager-migrate",
				"app.kuberentes.io/component: migrate",
				"helm.sh/hook: pre-install,pre-upgrade",
				"helm.sh/weight: \"0\"",
				"helm.sh/hook-delete-policy: before-hook-creation",
				"ttlSecondsAfterFinished: 300",
				"restartPolicy: OnFailure",
			},
		},
		{
			name:   "with custom image tag",
			values: "--set image.tag=v2.0.0",
			expected: []string{
				`image: "ghcr.io/openkcm/images/session-manager:v2.0.0"`,
			},
		},
		{
			name:   "with custom image pull policy",
			values: "--set image.pullPolicy=Always",
			expected: []string{
				"imagePullPolicy: Always",
			},
		},
		{
			name:   "with args",
			values: "",
			expected: []string{
				"args:",
				"migrate",
			},
		},
		{
			name:   "with empty env",
			values: "",
			expected: []string{
				"env: []",
			},
		},
		{
			name:   "with pod annotations",
			values: "--set pod.annotations.custom=annotation",
			expected: []string{
				"annotations:",
				"custom: annotation",
			},
		},
		{
			name:   "with pod labels",
			values: "--set pod.labels.environment=staging",
			expected: []string{
				"environment: staging",
			},
		},
		{
			name:   "with imagePullSecrets",
			values: "--set imagePullSecrets[0].name=registry-secret",
			expected: []string{
				"imagePullSecrets:",
				"name: registry-secret",
			},
		},
		{
			name:   "with pod securityContext",
			values: "--set pod.securityContext.fsGroup=1000",
			expected: []string{
				"securityContext:",
				"fsGroup: 1000",
			},
		},
		{
			name:   "with container securityContext",
			values: "--set securityContext.runAsNonRoot=true",
			expected: []string{
				"securityContext:",
				"runAsNonRoot: true",
			},
		},
		{
			name:   "with volumes and volumeMounts",
			values: "",
			expected: []string{
				"volumeMounts:",
				"name: session-manager-config-volume",
				"mountPath: /etc/session-manager",
				"readOnly: true",
				"volumes:",
				"projected:",
				"configMap:",
				"name: session-manager-config",
			},
		},
		{
			name:   "with extraVolumes",
			values: "--set extraVolumes[0].name=data-vol --set extraVolumes[0].emptyDir.medium=Memory",
			expected: []string{
				"name: data-vol",
				"emptyDir:",
				"medium: Memory",
			},
		},
		{
			name:   "with extraVolumeMounts",
			values: "--set extraVolumeMounts[0].name=data-mount --set extraVolumeMounts[0].mountPath=/data",
			expected: []string{
				"name: data-mount",
				"mountPath: /data",
			},
		},
		{
			name:   "with extraInitContainers",
			values: "--set extraInitContainers[0].name=wait-for-db --set extraInitContainers[0].image=busybox:latest",
			expected: []string{
				"initContainers:",
				"name: wait-for-db",
				"image: busybox:latest",
			},
		},
		{
			name:   "with extraContainers",
			values: "--set extraContainers[0].name=backup --set extraContainers[0].image=backup:1.0",
			expected: []string{
				"name: backup",
				"image: backup:1.0",
			},
		},
		{
			name:   "with custom image command",
			values: "--set image.command[0]=/custom/migrate",
			expected: []string{
				"command:",
				"/custom/migrate",
			},
		},
		{
			name:   "with custom image registry and repository",
			values: "--set image.registry=my-registry.io --set image.repository=custom/session-manager",
			expected: []string{
				`image: "my-registry.io/custom/session-manager:latest"`,
			},
		},
		{
			name:   "with custom image digest",
			values: "--set image.digest=sha256:abc123def456",
			expected: []string{
				"@sha256:abc123def456",
			},
		},
		{
			name:   "with component label",
			values: "",
			expected: []string{
				"app.kubernetes.io/component: migrate",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path, "-s", "templates/migrate/job.yaml"}
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
