//go:build helmtests

package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestServiceAccountRendering(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name       string
		values     string
		expected   []string
		unexpected []string
	}{
		{
			name:   "default service account",
			values: "",
			expected: []string{
				"kind: ServiceAccount",
				"name: session-manager",
				"automountServiceAccountToken: false",
			},
		},
		{
			name:   "custom service account name",
			values: "--set serviceAccount.name=custom-sa",
			expected: []string{
				"name: custom-sa",
			},
		},
		{
			name:   "with service account annotations",
			values: "--set serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn=arn:aws:iam::123456789:role/my-role",
			expected: []string{
				"annotations:",
				"eks.amazonaws.com/role-arn: arn:aws:iam::123456789:role/my-role",
			},
		},
		{
			// Session Manager does not call the Kubernetes API.
			name:   "without Role and RoleBinding",
			values: "",
			expected: []string{
				"namespace: default",
			},
			unexpected: []string{
				"kind: Role",
				"rbac.authorization.k8s.io",
			},
		},
		{
			name:   "with automount",
			values: "--set serviceAccount.automount=true",
			expected: []string{
				"automountServiceAccountToken: true",
			},
		},
		{
			name:   "with labels on ServiceAccount",
			values: "",
			expected: []string{
				"labels:",
				"app.kubernetes.io/name: session-manager",
				"app.kubernetes.io/instance: session-manager",
				"app.kubernetes.io/managed-by: Helm",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", appName, path, "-s", "templates/serviceaccount.yaml",
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
