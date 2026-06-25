//go:build helmtests

package main_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// logClusterStatus logs the current k8s cluster state for debugging.
// Call this whenever an error occurs to capture cluster state.
func logClusterStatus(t *testing.T, namespace string) {
	t.Helper()
	t.Log("=== CLUSTER STATUS (debugging info) ===")

	// Use a short timeout context for kubectl commands
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// All pods across all namespaces
	t.Log("--- All Pods (all namespaces) ---")
	out, _ := exec.CommandContext(ctx, "kubectl", "get", "pods", "-A", "-o", "wide").CombinedOutput()
	t.Log(string(out))

	// Services in the target namespace
	t.Logf("--- Services (namespace: %s) ---", namespace)
	out, _ = exec.CommandContext(ctx, "kubectl", "get", "services", "-n", namespace).CombinedOutput()
	t.Log(string(out))

	// Recent events sorted by timestamp
	t.Logf("--- Events (namespace: %s, sorted by lastTimestamp) ---", namespace)
	out, _ = exec.CommandContext(ctx, "kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp").CombinedOutput()
	t.Log(string(out))

	// Pod descriptions for detailed container status
	t.Logf("--- Pod Descriptions (namespace: %s) ---", namespace)
	out, _ = exec.CommandContext(ctx, "kubectl", "describe", "pods", "-n", namespace).CombinedOutput()
	t.Log(string(out))

	t.Log("=== END CLUSTER STATUS ===")
}

// getK8sClient creates a kubernetes clientset using default kubeconfig.
func getK8sClient(t *testing.T) *kubernetes.Clientset {
	t.Helper()

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		t.Fatalf("failed to get kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("failed to create kubernetes client: %v", err)
	}

	return clientset
}

// helmRepoAdd adds a helm repository.
func helmRepoAdd(ctx context.Context, t *testing.T, repoName, url string) error {
	t.Helper()
	cmd := exec.CommandContext(ctx, "helm", "repo", "add", repoName, url)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm repo add failed: %w\nOutput: %s", err, string(out))
	}
	t.Logf("Added helm repo %s: %s", repoName, url)
	return nil
}

// helmRepoRemove removes a helm repository.
func helmRepoRemove(ctx context.Context, t *testing.T, repoName string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "helm", "repo", "remove", repoName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Warning: helm repo remove %s failed: %v\nOutput: %s", repoName, err, string(out))
	}
}

// helmInstall installs a helm chart and returns error.
func helmInstall(ctx context.Context, t *testing.T, namespace, releaseName, chart string, values map[string]string, extraArgs ...string) error {
	t.Helper()
	// Preallocate args slice: 5 base args + 2 per value + extraArgs
	args := make([]string, 0, 5+2*len(values)+len(extraArgs))
	args = append(args, "install", releaseName, chart, "-n", namespace)
	for k, v := range values {
		args = append(args, "--set", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, "helm", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Logf("Running: helm %s", strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm install failed: %w\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	t.Logf("Installed helm release %s", releaseName)
	return nil
}

// helmDelete removes a helm release.
func helmDelete(ctx context.Context, t *testing.T, namespace, releaseName string) {
	t.Helper()
	args := []string{"uninstall", releaseName, "-n", namespace}

	cmd := exec.CommandContext(ctx, "helm", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Warning: helm uninstall %s failed: %v\nOutput: %s", releaseName, err, string(out))
		return
	}
	t.Logf("Deleted helm release %s", releaseName)
}

// waitForPodsWithLabel waits for pods matching the label selector to be created.
func waitForPodsWithLabel(ctx context.Context, t *testing.T, client *kubernetes.Clientset, namespace, labelSelector string) ([]corev1.Pod, error) {
	t.Helper()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for pods with label selector: %s", labelSelector)
		case <-ticker.C:
			pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				t.Logf("Error listing pods: %v, retrying...", err)
				continue
			}
			if len(pods.Items) > 0 {
				t.Logf("Found %d pod(s) with label selector: %s", len(pods.Items), labelSelector)
				return pods.Items, nil
			}
			t.Logf("No pods found yet with label selector: %s, retrying...", labelSelector)
		}
	}
}

// waitForPodReady waits for a specific pod to become ready or succeed (for Jobs).
func waitForPodReady(ctx context.Context, t *testing.T, client *kubernetes.Clientset, namespace, podName string) error {
	t.Helper()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for pod %s to be ready", podName)
		case <-ticker.C:
			pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				t.Logf("Error getting pod %s: %v, retrying...", podName, err)
				continue
			}

			// Job pods that completed successfully
			if pod.Status.Phase == corev1.PodSucceeded {
				t.Logf("Pod %s has completed successfully (phase: %s)", podName, pod.Status.Phase)
				return nil
			}

			// Failed pods should fail
			if pod.Status.Phase == corev1.PodFailed {
				return fmt.Errorf("pod %s has failed (phase: %s)", podName, pod.Status.Phase)
			}

			// Check if running and ready
			if isPodReady(pod) {
				t.Logf("Pod %s is available", podName)
				return nil
			}

			t.Logf("Pod %s is not available yet (phase: %s), retrying...", podName, pod.Status.Phase)
		}
	}
}

// isPodReady checks if a pod is in ready state.
func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// generateUniqueID generates a unique 8-character hex ID.
func generateUniqueID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// TestHelmInstall tests helm installation without using terratest.
// On any error, it logs the current k8s cluster status for debugging.
func TestHelmInstall(t *testing.T) {
	ctx := t.Context()
	namespace := "default"

	// Initialize k8s client
	client := getK8sClient(t)

	// Install Valkey
	t.Log("Adding Valkey helm repo")
	if err := helmRepoAdd(ctx, t, "valkey", "https://valkey.io/valkey-helm/"); err != nil {
		logClusterStatus(t, namespace)
		t.Fatalf("failed to add valkey repo: %v", err)
	}
	defer helmRepoRemove(ctx, t, "valkey")

	valkeyValues := map[string]string{"image.tag": "latest"}
	if err := helmInstall(ctx, t, namespace, "valkey", "valkey/valkey", valkeyValues); err != nil {
		logClusterStatus(t, namespace)
		t.Fatalf("failed to install valkey: %v", err)
	}
	defer helmDelete(ctx, t, namespace, "valkey")

	// Wait for Valkey pods
	valkeyCtx, valkeyCancel := context.WithTimeout(ctx, 30*time.Second)
	defer valkeyCancel()
	valkeyPods, err := waitForPodsWithLabel(valkeyCtx, t, client, namespace, "app.kubernetes.io/name=valkey")
	if err != nil {
		logClusterStatus(t, namespace)
		t.Fatalf("failed waiting for valkey pods: %v", err)
	}
	for _, pod := range valkeyPods {
		t.Logf("Checking Valkey pod: %s", pod.Name)
		if err := waitForPodReady(valkeyCtx, t, client, namespace, pod.Name); err != nil {
			logClusterStatus(t, namespace)
			t.Fatalf("valkey pod not ready: %v", err)
		}
	}

	// Install PostgreSQL
	t.Log("Adding Bitnami helm repo")
	if err := helmRepoAdd(ctx, t, "bitnami", "https://charts.bitnami.com/bitnami"); err != nil {
		logClusterStatus(t, namespace)
		t.Fatalf("failed to add bitnami repo: %v", err)
	}
	defer helmRepoRemove(ctx, t, "bitnami")

	postgresValues := map[string]string{
		"auth.database":        "session_manager",
		"auth.username":        "postgres",
		"auth.password":        "postgres",
		"primary.service.type": "ClusterIP",
	}
	if err := helmInstall(ctx, t, namespace, "postgresql", "bitnami/postgresql", postgresValues); err != nil {
		logClusterStatus(t, namespace)
		t.Fatalf("failed to install postgresql: %v", err)
	}
	defer helmDelete(ctx, t, namespace, "postgresql")

	// Wait for PostgreSQL pods
	postgresCtx, postgresCancel := context.WithTimeout(ctx, 60*time.Second)
	defer postgresCancel()
	postgresPods, err := waitForPodsWithLabel(postgresCtx, t, client, namespace, "app.kubernetes.io/name=postgresql")
	if err != nil {
		logClusterStatus(t, namespace)
		t.Fatalf("failed waiting for postgresql pods: %v", err)
	}
	for _, pod := range postgresPods {
		t.Logf("Checking PostgreSQL pod: %s", pod.Name)
		if err := waitForPodReady(postgresCtx, t, client, namespace, pod.Name); err != nil {
			logClusterStatus(t, namespace)
			t.Fatalf("postgresql pod not ready: %v", err)
		}
	}

	// Give databases a moment to fully initialize
	t.Log("Waiting 5 more seconds for databases to fully initialize")
	time.Sleep(5 * time.Second)

	// Install session-manager
	releaseName := fmt.Sprintf("%s-%s", app, strings.ToLower(generateUniqueID()))
	sessionManagerValues := map[string]string{
		"namespace":                                           "default",
		"image.registry":                                      "localhost",
		"image.repository":                                    "session-manager",
		"image.tag":                                           "latest",
		"image.pullPolicy":                                    "Never",
		"config.database.host.value":                          "postgresql.default.svc.cluster.local",
		"config.database.user.value":                          "postgres",
		"config.database.password.value":                      "postgres",
		"config.valkey.host.value":                            "valkey.default.svc.cluster.local:6379",
		"config.valkey.password.value":                        "",
		"config.sessionManager.callbackURL":                   "http://localhost:8080/sm/callback",
		"config.sessionManager.clientAuth.clientID":           "test-client",
		"config.sessionManager.clientAuth.clientSecret.value": "test-secret",
		"config.sessionManager.csrfSecret.value":              "test-csrf-secret-at-least-thirty-two-bits",
	}
	if err := helmInstall(ctx, t, namespace, releaseName, path, sessionManagerValues, "--timeout", "12m", "--wait"); err != nil {
		logClusterStatus(t, namespace)
		t.Fatalf("failed to install session-manager: %v", err)
	}
	defer helmDelete(ctx, t, namespace, releaseName)

	// Verify deployment
	t.Log("Verifying deployment after helm install completed")

	// Get all session-manager pods
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + app,
	})
	if err != nil {
		logClusterStatus(t, namespace)
		t.Fatalf("failed to list pods: %v", err)
	}
	t.Logf("Found %d pod(s) after installation", len(pods.Items))

	// Check each pod's status
	for _, pod := range pods.Items {
		t.Logf("Pod %s: phase=%s", pod.Name, pod.Status.Phase)

		// Log container logs for each pod
		for _, container := range pod.Spec.Containers {
			logOpts := &corev1.PodLogOptions{
				Container: container.Name,
			}
			req := client.CoreV1().Pods(namespace).GetLogs(pod.Name, logOpts)
			logStream, err := req.Stream(ctx)
			if err != nil {
				t.Logf("Failed to get logs for pod %s container %s: %v", pod.Name, container.Name, err)
				continue
			}
			var logBuf bytes.Buffer
			_, _ = io.Copy(&logBuf, logStream)
			logStream.Close()
			t.Logf("Logs for pod %s container %s:\n%s", pod.Name, container.Name, logBuf.String())
		}

		// Job pods should be Succeeded
		if strings.Contains(pod.Name, "migrate") {
			if pod.Status.Phase != corev1.PodSucceeded {
				logClusterStatus(t, namespace)
				t.Errorf("Expected migrate job pod %s to be Succeeded, got %s", pod.Name, pod.Status.Phase)
			}
		} else {
			// Regular pods should be Running
			if pod.Status.Phase != corev1.PodRunning || !isPodReady(&pod) {
				logClusterStatus(t, namespace)
				t.Errorf("Expected pod %s to be Running and available, got phase=%s available=%v",
					pod.Name, pod.Status.Phase, isPodReady(&pod))
			}
		}
	}

	// Verify services exist
	t.Log("Verifying services")
	service, err := client.CoreV1().Services(namespace).Get(ctx, releaseName, metav1.GetOptions{})
	if err != nil {
		logClusterStatus(t, namespace)
		t.Fatalf("Expected service to exist: %v", err)
	}
	t.Logf("Service %s exists with type %s", service.Name, service.Spec.Type)
}
