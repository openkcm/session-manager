//go:build helmtests

package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestMigrateJobRendering(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name       string
		values     string
		expected   []string
		unexpected []string
	}{
		{
			name:   "default migrate job",
			values: "",
			expected: []string{
				"kind: Job",
				"name: session-manager-migrate",
				"app.kubernetes.io/component: migrate",
				"helm.sh/hook: pre-install,pre-upgrade",
				"helm.sh/hook-weight: \"0\"",
				"helm.sh/hook-delete-policy: before-hook-creation",
				"ttlSecondsAfterFinished: 300",
				"backoffLimit: 6",
				"restartPolicy: OnFailure",
				"ephemeral-storage: 128Mi",
				"ephemeral-storage: 512Mi",
				"automountServiceAccountToken: false",
				"runAsUser: 65532",
				"readOnlyRootFilesystem: true",
			},
			// The chart's ServiceAccount does not exist yet when the hook runs.
			unexpected: []string{
				"app.kuberentes.io",
				"serviceAccountName:",
				"activeDeadlineSeconds:",
			},
		},
		{
			name:   "with a ServiceAccount that exists before the installation",
			values: "--set serviceAccount.create=false --set serviceAccount.name=existing-sa",
			expected: []string{
				"serviceAccountName: existing-sa",
			},
		},
		{
			name:   "with job settings",
			values: "--set migrate.backoffLimit=2 --set migrate.activeDeadlineSeconds=600 --set migrate.ttlSecondsAfterFinished=0",
			expected: []string{
				"backoffLimit: 2",
				"activeDeadlineSeconds: 600",
				"ttlSecondsAfterFinished: 0",
			},
		},
		{
			name:   "without ttlSecondsAfterFinished",
			values: "--set migrate.ttlSecondsAfterFinished=null",
			unexpected: []string{
				"ttlSecondsAfterFinished",
			},
		},
		{
			name:   "with migrate.podAnnotations",
			values: "--set migrate.podAnnotations.sidecar\\.istio\\.io/inject=false --set pod.annotations.team=kms",
			expected: []string{
				`sidecar.istio.io/inject: "false"`,
				`team: "kms"`,
			},
		},
		{
			name:   "with migrate.resources",
			values: "--set migrate.resources.limits.memory=300Mi",
			expected: []string{
				"memory: 300Mi",
			},
		},
		{
			name:   "with custom image tag",
			values: "--set image.tag=v2.0.0",
			expected: []string{
				`image: "ghcr.io/openkcm/session-manager/images/session-manager:v2.0.0"`,
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
			name:   "with extra environment variables",
			values: "--set extraEnvs[0].name=PGSSLMODE --set extraEnvs[0].value=verify-full",
			expected: []string{
				"name: PGSSLMODE",
				"value: verify-full",
			},
		},
		{
			name:   "with pod annotations",
			values: "--set pod.annotations.custom=annotation",
			expected: []string{
				"annotations:",
				`custom: "annotation"`,
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
				"name: config",
				"mountPath: /etc/session-manager",
				"readOnly: true",
				"volumes:",
				"configMap:",
				// The job uses the hook copy of the configuration.
				"name: session-manager-config",
			},
			unexpected: []string{
				"projected:",
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
			name:   "without the extraContainers of the deployments",
			values: "--set extraContainers[0].name=backup --set extraContainers[0].image=backup:1.0",
			// A sidecar that keeps running would keep the job from completing.
			unexpected: []string{
				"name: backup",
			},
		},
		{
			name:   "with migrate.extraContainers",
			values: "--set migrate.extraContainers[0].name=backup --set migrate.extraContainers[0].image=backup:1.0",
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
			args := []string{"template", appName, path, "-s", "templates/migrate/job.yaml",
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

func TestMigrateConfigMapRendering(t *testing.T) {
	ctx := t.Context()
	args := []string{"template", appName, path, "-s", "templates/migrate/configmap.yaml",
		"--set", "image.tag=latest",
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
	// The job runs before the release's resources exist, so its configuration is a hook
	// that runs first and is deleted when the migration succeeds.
	for _, expected := range []string{
		"kind: ConfigMap",
		"name: session-manager-config",
		"app.kubernetes.io/component: migrate",
		"helm.sh/hook: pre-install,pre-upgrade",
		"helm.sh/hook-weight: \"-1\"",
		"helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded",
		"config.yaml:",
		"dbModule: database.module.pgxpool",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput: %s", expected, output)
		}
	}
}
