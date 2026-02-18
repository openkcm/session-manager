package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestServiceAccountRendering(t *testing.T) {
	tests := []struct {
		name     string
		values   string
		expected []string
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
			name:   "with Role created",
			values: "",
			expected: []string{
				"kind: Role",
				"name: session-manager",
				"namespace: default",
				"rules: []",
			},
		},
		{
			name:   "with RoleBinding created",
			values: "",
			expected: []string{
				"kind: RoleBinding",
				"name: session-manager",
				"namespace: default",
				"subjects:",
				"kind: ServiceAccount",
				"roleRef:",
				"kind: Role",
				"apiGroup: rbac.authorization.k8s.io",
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
			args := []string{"template", appName, path, "-s", "templates/serviceaccount.yaml"}
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
