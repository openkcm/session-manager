package main_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHelmInstall(t *testing.T) {
	// Create required k8s resources
	kubeOpts := k8s.NewKubectlOptions("", "", "default")

	// Install Valkey
	valkeyOptions := &helm.Options{
		SetValues: map[string]string{
			"image.tag": "latest",
		},
	}
	helm.AddRepo(t, valkeyOptions, "valkey", "https://valkey.io/valkey-helm/")
	defer helm.RemoveRepo(t, valkeyOptions, "valkey")
	helm.Install(t, valkeyOptions, "valkey/valkey", "valkey")
	defer helm.Delete(t, valkeyOptions, "valkey", true)
	valkeyCtx, valkeyCancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer valkeyCancel()
	valkeyPods := waitForPodCreation(valkeyCtx, t, kubeOpts, "app.kubernetes.io/name=valkey")
	for _, pod := range valkeyPods {
		t.Logf("Checking Valkey pod: %s", pod.Name)
		waitForPodAvailability(valkeyCtx, t, kubeOpts, pod.Name)
	}

	// Install PostgreSQL
	postgresOptions := &helm.Options{
		SetValues: map[string]string{
			"auth.database":        "session_manager",
			"auth.username":        "postgres",
			"auth.password":        "postgres",
			"primary.service.type": "ClusterIP",
		},
	}
	helm.AddRepo(t, postgresOptions, "bitnami", "https://charts.bitnami.com/bitnami")
	defer helm.RemoveRepo(t, postgresOptions, "bitnami")
	helm.Install(t, postgresOptions, "bitnami/postgresql", "postgresql")
	defer helm.Delete(t, postgresOptions, "postgresql", true)
	postgresCtx, postgresCancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer postgresCancel()
	postgresPods := waitForPodCreation(postgresCtx, t, kubeOpts, "app.kubernetes.io/name=postgresql")
	for _, pod := range postgresPods {
		t.Logf("Checking PostgreSQL pod: %s", pod.Name)
		waitForPodAvailability(postgresCtx, t, kubeOpts, pod.Name)
	}

	// Give databases a moment to fully initialize
	t.Log("Waiting for databases to fully initialize")
	time.Sleep(5 * time.Second)

	// Create the helm options for session-manager
	helmOpts := &helm.Options{
		SetValues: map[string]string{
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
			"config.sessionManager.clientAuth.clientSecret.value": "test-secret",
			"config.sessionManager.csrfSecret.value":              "test-csrf-secret-at-least-thirty-two-bits",
		},
		ExtraArgs: map[string][]string{
			"install": {"--timeout", "12m", "--wait"},
		},
	}
	releaseName := fmt.Sprintf("%s-%s", app, strings.ToLower(random.UniqueId()))

	// Act: Install session-manager
	// The --wait flag means helm will wait for all resources to be ready before returning
	helm.Install(t, helmOpts, path, releaseName)
	defer helm.Delete(t, helmOpts, releaseName, true)

	// Helm --wait should ensure resources are ready, but let's verify pods exist and are in good state
	t.Log("Verifying deployment after helm install completed")

	// Get all pods
	pods := k8s.ListPods(t, kubeOpts, metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=" + app})
	t.Logf("Found %d pod(s) after installation", len(pods))

	// Check each pod's status
	for _, pod := range pods {
		t.Logf("Pod %s: phase=%s", pod.Name, pod.Status.Phase)
		// Job pods should be Succeeded
		if strings.Contains(pod.Name, "migrate") {
			if pod.Status.Phase != corev1.PodSucceeded {
				t.Errorf("Expected migrate job pod %s to be Succeeded, got %s", pod.Name, pod.Status.Phase)
			}
		} else {
			// Regular pods should be Running
			if pod.Status.Phase != corev1.PodRunning || !k8s.IsPodAvailable(&pod) {
				t.Errorf("Expected pod %s to be Running and available, got phase=%s available=%v",
					pod.Name, pod.Status.Phase, k8s.IsPodAvailable(&pod))
			}
		}
	}

	// Verify services exist
	t.Log("Verifying services")
	// The service name should match the release name
	service := k8s.GetService(t, kubeOpts, releaseName)
	if service == nil {
		t.Fatal("Expected service to exist")
	}
	t.Logf("Service %s exists with type %s", service.Name, service.Spec.Type)
}

func waitForPodCreation(ctx context.Context, t *testing.T, kubeOpts *k8s.KubectlOptions, labelSelector string) []corev1.Pod {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for pod creation with label selector: %s", labelSelector)
		default:
			pods := k8s.ListPods(t, kubeOpts, metav1.ListOptions{LabelSelector: labelSelector})
			if len(pods) > 0 {
				t.Logf("Found %d pod(s) with label selector: %s", len(pods), labelSelector)
				return pods
			}
			t.Logf("No pods found yet with label selector: %s, retrying...", labelSelector)
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func waitForPodAvailability(ctx context.Context, t *testing.T, kubeOpts *k8s.KubectlOptions, podName string) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for pod availability: %s", podName)
		default:
			pod := k8s.GetPod(t, kubeOpts, podName)

			// Job pods that have completed successfully should be considered "done" and not waiting
			if pod.Status.Phase == corev1.PodSucceeded {
				t.Logf("Pod %s has completed successfully (phase: %s)", podName, pod.Status.Phase)
				return
			}

			// Failed pods should fail the test
			if pod.Status.Phase == corev1.PodFailed {
				t.Fatalf("Pod %s has failed (phase: %s)", podName, pod.Status.Phase)
			}

			if k8s.IsPodAvailable(pod) {
				t.Logf("Pod %s is available", podName)
				return
			}
			t.Logf("Pod %s is not available yet (phase: %s), retrying...", podName, pod.Status.Phase)
			time.Sleep(500 * time.Millisecond)
		}
	}
}
