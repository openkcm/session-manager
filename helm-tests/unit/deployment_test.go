//go:build helmtests

package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestDeploymentRendering(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name       string
		values     string
		expected   []string
		unexpected []string
	}{
		{
			name:   "default values",
			values: "",
			expected: []string{
				"kind: Deployment",
				"name: session-manager",
				"app.kubernetes.io/name: session-manager",
				"app.kubernetes.io/component: session-manager",
				"replicas: 1",
				"namespace: default",
				"checksum/config:",
				"automountServiceAccountToken: false",
				"enableServiceLinks: false",
				"runAsNonRoot: true",
				"runAsUser: 65532",
				"runAsGroup: 65532",
				"fsGroup: 65532",
				"readOnlyRootFilesystem: true",
				"allowPrivilegeEscalation: false",
				"mountPath: /tmp",
				`args: ["api-server"]`,
				"ephemeral-storage: 256Mi",
				"ephemeral-storage: 1Gi",
			},
			unexpected: []string{
				"command:",
				"initContainers:",
			},
		},
		{
			name:   "custom replica count",
			values: "--set replicaCount=3",
			expected: []string{
				"replicas: 3",
			},
		},
		{
			name:   "image tag defaults to the chart's appVersion",
			values: "--set image.tag=",
			expected: []string{
				`image: "ghcr.io/openkcm/session-manager/images/session-manager:` + appVersion(t) + `"`,
			},
		},
		{
			name:   "custom image tag",
			values: "--set image.tag=v1.2.3",
			expected: []string{
				`image: "ghcr.io/openkcm/session-manager/images/session-manager:v1.2.3"`,
			},
		},
		{
			name:   "image digest before tag",
			values: "--set image.tag=v1.2.3 --set image.digest=sha256:abc",
			expected: []string{
				`image: "ghcr.io/openkcm/session-manager/images/session-manager@sha256:abc"`,
			},
		},
		{
			name:   "custom pull policy",
			values: "--set image.pullPolicy=Always",
			expected: []string{
				"imagePullPolicy: Always",
			},
		},
		{
			name:   "replicas omitted when autoscaling enabled",
			values: "--set autoscaling.enabled=true",
			expected: []string{
				"kind: Deployment",
				"selector:",
			},
			unexpected: []string{
				"replicas:",
			},
		},
		{
			name:   "custom pod annotations",
			values: "--set pod.annotations.prometheus\\.io/scrape=true --set pod.annotations.prometheus\\.io/port=8080",
			expected: []string{
				"annotations:",
				// Annotation values are always rendered as strings.
				`prometheus.io/port: "8080"`,
				`prometheus.io/scrape: "true"`,
			},
		},
		{
			name:   "custom pod labels",
			values: "--set pod.labels.team=backend --set pod.labels.env=prod",
			expected: []string{
				"env: prod",
				"team: backend",
			},
		},
		{
			name:   "custom service account",
			values: "--set serviceAccount.name=custom-sa",
			expected: []string{
				"serviceAccountName: custom-sa",
			},
		},
		{
			name:   "with imagePullSecrets",
			values: "--set imagePullSecrets[0].name=my-secret",
			expected: []string{
				"imagePullSecrets:",
				"name: my-secret",
			},
		},
		{
			name:   "with resources",
			values: "--set resources.limits.cpu=1000m --set resources.limits.memory=512Mi --set resources.requests.cpu=100m --set resources.requests.memory=128Mi",
			expected: []string{
				"resources:",
				"limits:",
				"cpu: 1000m",
				"memory: 512Mi",
				"requests:",
				"cpu: 100m",
				"memory: 128Mi",
			},
		},
		{
			// The API server is ready when its gRPC port listens: the status server's
			// readiness endpoint reports ready before the API server listens.
			name:   "with liveness and readiness probes",
			values: "",
			expected: []string{
				"livenessProbe:",
				"path: /probe/liveness",
				"port: http-status",
				"tcpSocket:\n              port: grpc\n",
			},
			unexpected: []string{
				"path: /probe/readiness",
			},
		},
		{
			name:   "with environment variables",
			values: "",
			expected: []string{
				"env:",
				"name: MY_POD_IP",
				"name: K8S_NODE_NAME",
				"name: K8S_NODE_IP",
				"fieldRef:",
			},
		},
		{
			name:   "with extra environment variables",
			values: "--set extraEnvs[0].name=CUSTOM_VAR --set extraEnvs[0].value=custom-value",
			expected: []string{
				"name: CUSTOM_VAR",
				"value: custom-value",
			},
		},
		{
			name:   "with volumes and volumeMounts",
			values: "",
			expected: []string{
				"volumeMounts:",
				"name: config",
				"mountPath: /etc/session-manager/config.yaml\n",
				"subPath: config.yaml",
				"readOnly: true",
				"volumes:",
				"configMap:",
				"name: tmp",
				"emptyDir:",
			},
			unexpected: []string{
				"projected:",
				"session-manager-config",
			},
		},
		{
			name:   "configuration path does not depend on nameOverride",
			values: "--set nameOverride=custom-name",
			expected: []string{
				"mountPath: /etc/session-manager/config.yaml\n",
				"subPath: config.yaml",
				"app.kubernetes.io/name: custom-name",
			},
			unexpected: []string{
				"mountPath: /etc/custom-name",
			},
		},
		{
			name:   "with image command as a string",
			values: "--set image.command=/custom/command",
			expected: []string{
				"command:",
				`- "/custom/command"`,
			},
		},
		{
			name:   "with lifecycle",
			values: "--set lifecycle.preStop.sleep.seconds=5",
			expected: []string{
				"lifecycle:",
				"preStop:",
				"seconds: 5",
			},
		},
		{
			name:   "with topologySpreadConstraints",
			values: "--set topologySpreadConstraints[0].maxSkew=1 --set topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
			expected: []string{
				"topologySpreadConstraints:",
				"topologyKey: topology.kubernetes.io/zone",
			},
		},
		{
			name:   "with extraVolumes",
			values: "--set extraVolumes[0].name=extra-vol --set extraVolumes[0].emptyDir.medium=Memory",
			expected: []string{
				"name: extra-vol",
				"emptyDir:",
				"medium: Memory",
			},
		},
		{
			name:   "with extraVolumeMounts",
			values: "--set extraVolumeMounts[0].name=extra-mount --set extraVolumeMounts[0].mountPath=/mnt/extra",
			expected: []string{
				"name: extra-mount",
				"mountPath: /mnt/extra",
			},
		},
		{
			name:   "with nodeSelector",
			values: "--set nodeSelector.disktype=ssd --set nodeSelector.zone=us-west",
			expected: []string{
				"nodeSelector:",
				"disktype: ssd",
				"zone: us-west",
			},
		},
		{
			name:   "with tolerations",
			values: "--set tolerations[0].key=node.kubernetes.io/not-ready --set tolerations[0].operator=Exists --set tolerations[0].effect=NoExecute",
			expected: []string{
				"tolerations:",
				"key: node.kubernetes.io/not-ready",
				"operator: Exists",
				"effect: NoExecute",
			},
		},
		{
			name:   "with affinity",
			values: "--set affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key=kubernetes.io/hostname",
			expected: []string{
				"affinity:",
				"nodeAffinity:",
			},
		},
		{
			name:   "with ports",
			values: "",
			expected: []string{
				"ports:",
				"containerPort: 8888",
				"name: http-status",
				"containerPort: 8080",
				"name: http",
				"containerPort: 9091",
				"name: grpc",
			},
		},
		{
			name:   "with custom image command",
			values: "--set image.command[0]=/custom/command",
			expected: []string{
				"command:",
				"/custom/command",
			},
		},
		{
			name:   "with args",
			values: "",
			expected: []string{
				"args:",
				"api-server",
			},
		},
		{
			name:   "with extraEnvsFrom",
			values: "--set extraEnvsFrom[0].configMapRef.name=my-config",
			expected: []string{
				"envFrom:",
				"configMapRef:",
				"name: my-config",
			},
		},
		{
			name:   "with extraInitContainers",
			values: "--set extraInitContainers[0].name=init-container --set extraInitContainers[0].image=busybox",
			expected: []string{
				"initContainers:",
				"name: init-container",
				"image: busybox",
			},
		},
		{
			name:   "with extraContainers",
			values: "--set extraContainers[0].name=sidecar --set extraContainers[0].image=sidecar:latest",
			expected: []string{
				"name: sidecar",
				"image: sidecar:latest",
			},
		},
		{
			name:   "with pod securityContext",
			values: "--set pod.securityContext.fsGroup=2000 --set pod.securityContext.runAsUser=1000",
			expected: []string{
				"securityContext:",
				"fsGroup: 2000",
				"runAsUser: 1000",
			},
		},
		{
			name:   "with container securityContext",
			values: "--set securityContext.runAsNonRoot=true --set securityContext.readOnlyRootFilesystem=true",
			expected: []string{
				"securityContext:",
				"readOnlyRootFilesystem: true",
				"runAsNonRoot: true",
			},
		},
		{
			name:   "with custom image digest",
			values: "--set image.digest=sha256:abc123",
			expected: []string{
				"@sha256:abc123",
			},
		},
		{
			name:   "with custom image registry and repository",
			values: "--set image.registry=docker.io --set image.repository=myorg/session-manager",
			expected: []string{
				`image: "docker.io/myorg/session-manager:latest"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path, "-s", "templates/session-manager/deployment.yaml",
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

func TestHousekeeperDeployment(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name       string
		values     string
		expected   []string
		unexpected []string
	}{
		{
			name:   "default housekeeper deployment",
			values: "",
			expected: []string{
				"kind: Deployment",
				"name: session-manager-housekeeper",
				"app.kubernetes.io/component: housekeeper",
				"replicas: 1",
				"args:",
				"housekeeper",
			},
		},
		{
			name:   "housekeeper with default resources",
			values: "",
			expected: []string{
				"resources:",
				"limits:",
				"memory: 256Mi",
				"ephemeral-storage: 512Mi",
				"requests:",
				"cpu: 50m",
				"memory: 64Mi",
				"ephemeral-storage: 128Mi",
			},
		},
		{
			name:   "housekeeper with custom resources",
			values: "--set housekeeper.resources.limits.cpu=200m --set housekeeper.resources.limits.memory=300Mi",
			expected: []string{
				"resources:",
				"limits:",
				"cpu: 200m",
				"memory: 300Mi",
			},
		},
		{
			name:   "housekeeper does not use the API server resources",
			values: "--set resources.limits.cpu=900m",
			unexpected: []string{
				"cpu: 900m",
			},
		},
		{
			// A rolling update without surge stops the old pod before the new one starts.
			// Recreate would fail upgrades from chart versions before 0.11.0 with
			// server-side apply: the Deployment keeps its rollingUpdate settings.
			name:   "housekeeper probes, status port and update strategy",
			values: "",
			expected: []string{
				"strategy:\n    rollingUpdate:\n      maxSurge: 0\n      maxUnavailable: 1\n    type: RollingUpdate\n",
				"checksum/config:",
				"containerPort: 8888",
				"name: http-status",
				"path: /probe/liveness",
				"path: /probe/readiness",
				"runAsUser: 65532",
				"fsGroup: 65532",
				"readOnlyRootFilesystem: true",
			},
			// The housekeeper serves only the status endpoints.
			unexpected: []string{
				"containerPort: 8080",
				"containerPort: 9091",
				"Recreate",
			},
		},
		{
			name:   "housekeeper with volumes",
			values: "",
			expected: []string{
				"volumeMounts:",
				"name: config",
				"mountPath: /etc/session-manager/config.yaml\n",
				"subPath: config.yaml",
				"volumes:",
				"configMap:",
				"mountPath: /tmp",
			},
			unexpected: []string{
				"projected:",
			},
		},
		{
			name:   "housekeeper with custom image",
			values: "--set image.tag=v2.0.0",
			expected: []string{
				`image: "ghcr.io/openkcm/session-manager/images/session-manager:v2.0.0"`,
			},
		},
		{
			name:   "housekeeper with pod annotations",
			values: "--set pod.annotations.custom=value",
			expected: []string{
				"annotations:",
				`custom: "value"`,
			},
		},
		{
			name:   "housekeeper with nodeSelector",
			values: "--set nodeSelector.worker-type=housekeeper",
			expected: []string{
				"nodeSelector:",
				"worker-type: housekeeper",
			},
		},
		{
			name:   "housekeeper with extraVolumes",
			values: "--set extraVolumes[0].name=extra-vol --set extraVolumes[0].emptyDir.medium=Memory",
			expected: []string{
				"name: extra-vol",
				"emptyDir:",
				"medium: Memory",
			},
		},
		{
			name:   "housekeeper with extraVolumeMounts",
			values: "--set extraVolumeMounts[0].name=extra-mount --set extraVolumeMounts[0].mountPath=/mnt/data",
			expected: []string{
				"name: extra-mount",
				"mountPath: /mnt/data",
			},
		},
		{
			name:   "housekeeper with extraInitContainers",
			values: "--set extraInitContainers[0].name=init-db --set extraInitContainers[0].image=init:1.0",
			expected: []string{
				"initContainers:",
				"name: init-db",
				"image: init:1.0",
			},
		},
		{
			name:   "housekeeper with extraContainers",
			values: "--set extraContainers[0].name=metrics --set extraContainers[0].image=metrics:latest",
			expected: []string{
				"name: metrics",
				"image: metrics:latest",
			},
		},
		{
			name:   "housekeeper with pod securityContext",
			values: "--set pod.securityContext.runAsUser=1000",
			expected: []string{
				"securityContext:",
				"runAsUser: 1000",
			},
		},
		{
			name:   "housekeeper with container securityContext",
			values: "--set securityContext.runAsNonRoot=true",
			expected: []string{
				"securityContext:",
				"runAsNonRoot: true",
			},
		},
		{
			name:   "housekeeper with imagePullSecrets",
			values: "--set imagePullSecrets[0].name=registry-secret",
			expected: []string{
				"imagePullSecrets:",
				"name: registry-secret",
			},
		},
		{
			name:   "housekeeper with pod labels",
			values: "--set pod.labels.component=housekeeper --set pod.labels.env=production",
			expected: []string{
				"component: housekeeper",
				"env: production",
			},
		},
		{
			name:   "housekeeper replicas always 1",
			values: "--set replicaCount=5",
			expected: []string{
				"replicas: 1",
			},
		},
		{
			name:   "housekeeper with custom image command",
			values: "--set image.command[0]=/bin/custom-housekeeper",
			expected: []string{
				"command:",
				"/bin/custom-housekeeper",
			},
		},
		{
			name:   "housekeeper with extra environment variables",
			values: "--set extraEnvs[0].name=PGSSLMODE --set extraEnvs[0].value=verify-full --set extraEnvsFrom[0].secretRef.name=db-env",
			expected: []string{
				"name: PGSSLMODE",
				"value: verify-full",
				"envFrom:",
				"name: db-env",
			},
		},
		{
			name:   "housekeeper with affinity",
			values: "--set housekeeper.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key=pool",
			expected: []string{
				"affinity:",
				"key: pool",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path, "-s", "templates/housekeeper/deployment.yaml",
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
