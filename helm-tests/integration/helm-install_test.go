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
			"image.registry":                                      "ghcr.io/openkcm",
			"image.repository":                                    "images/session-manager",
			"image.tag":                                           "latest",
			"config.database.host.value":                          "postgresql.default.svc.cluster.local",
			"config.database.user.value":                          "postgres",
			"config.database.password.value":                      "postgres",
			"config.valkey.host.value":                            "valkey-master.default.svc.cluster.local",
			"config.valkey.password.value":                        "",
			"config.sessionManager.callbackURL":                   "http://localhost:8080/sm/callback",
			"config.sessionManager.clientAuth.clientID":           "test-client",
			"config.sessionManager.clientAuth.clientSecret.value": "test-secret",
			"config.sessionManager.csrfSecret.value":              "test-csrf-secret-at-least-thirty-two-bits",
		},
	}
	releaseName := fmt.Sprintf("%s-%s", app, strings.ToLower(random.UniqueId()))

	// Act: Install session-manager
	helm.Install(t, helmOpts, path, releaseName)
	defer helm.Delete(t, helmOpts, releaseName, true)

	// Assert: Wait for pod creation
	ctx, cancel1 := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel1()

	pods := waitForPodCreation(ctx, t, kubeOpts, "app.kubernetes.io/name="+app)

	// Wait for pod availability
	for _, pod := range pods {
		t.Logf("Checking session-manager pod: %s", pod.Name)
		waitForPodAvailability(ctx, t, kubeOpts, pod.Name)
	}

	// Recheck pod availability after a short delay
	t.Log("Rechecking pod availability after a short delay")
	time.Sleep(5 * time.Second)
	for _, pod := range pods {
		t.Logf("Rechecking pod: %s", pod.Name)
		// Get pod logs for debugging
		//nolint:errcheck
		k8s.GetPodLogsE(t, kubeOpts, &pod, app)
		waitForPodAvailability(ctx, t, kubeOpts, pod.Name)
	}

	// Verify services exist
	t.Log("Verifying services")
	service := k8s.GetService(t, kubeOpts, releaseName+"-"+app)
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
			time.Sleep(100 * time.Millisecond)
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
			if k8s.IsPodAvailable(pod) {
				t.Logf("Pod %s is available", podName)
				return
			}
			t.Logf("Pod %s is not available yet (phase: %s), retrying...", podName, pod.Status.Phase)
			time.Sleep(250 * time.Millisecond)
		}
	}
}
