package controller

import (
	"context"
	"testing"
	"time"

	cleanupconfig "github.com/infrautils/kubeclean/internal/cleanup_config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPodCleanupController(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	// Test data: 2 pods with same label, different creation times
	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(oldPod, newPod).Build()

	cleanupCfg := &cleanupconfig.CleanupConfig{
		BatchSize: 2,
		DryRun:    false,
		PodCleanupConfig: cleanupconfig.PodCleanupConfig{
			Enabled: true,

			Rules: []cleanupconfig.PodCleanRule{
				{
					Name:    "succeeded-pods",
					Enabled: true,
					Phase:   string(corev1.PodSucceeded),
					TTL:     cleanupconfig.Duration{Duration: time.Hour},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Namespaces: []string{"default"},
				},
			},
		},
	}

	controller := NewPodCleanController(client, scheme, cleanupCfg)
	ctx := context.Background()

	// Run cleanup
	controller.RunCleanUp(ctx)

	// Validate that oldPod is deleted, newPod remains
	podList := &corev1.PodList{}
	if err := client.List(ctx, podList); err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(podList.Items) != 1 || podList.Items[0].Name != "new-pod" {
		t.Errorf("Unexpected pods after cleanup: %+v", podList.Items)
	}
}

func TestPodCleanupDryRun(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "dry-run-pod",
			Namespace:         "default",
			Labels:            map[string]string{"app": "test"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(oldPod).Build()

	cleanupCfg := &cleanupconfig.CleanupConfig{
		BatchSize: 1,
		DryRun:    true, // DRY RUN
		PodCleanupConfig: cleanupconfig.PodCleanupConfig{
			Enabled: true,

			Rules: []cleanupconfig.PodCleanRule{
				{
					Name:    "dry-run-rule",
					Enabled: true,
					Phase:   string(corev1.PodSucceeded),
					TTL:     cleanupconfig.Duration{Duration: time.Hour},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					Namespaces: []string{"default"},
				},
			},
		},
	}

	controller := NewPodCleanController(client, scheme, cleanupCfg)
	ctx := context.Background()

	// Run dry-run cleanup
	controller.RunCleanUp(ctx)

	pod := &corev1.Pod{}

	// Pod should remain
	if err := client.Get(ctx, ctrlclient.ObjectKey{Namespace: "default", Name: "dry-run-pod"}, pod); err != nil {
		t.Errorf("Pod should not be deleted in dry-run, got error: %v", err)
	}

}

func TestRunPodCleanJob(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job-pod",
			Namespace:         "default",
			Labels:            map[string]string{"app": "test"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pod).Build()

	cleanupCfg := &cleanupconfig.CleanupConfig{
		BatchSize: 1,
		DryRun:    false,
		PodCleanupConfig: cleanupconfig.PodCleanupConfig{
			Enabled: true,

			Rules: []cleanupconfig.PodCleanRule{
				{
					Name:    "job-test-rule",
					Enabled: true,
					Phase:   string(corev1.PodSucceeded),
					TTL:     cleanupconfig.Duration{Duration: time.Hour},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					Namespaces: []string{"default"},
				},
			},
		},
	}

	controller := NewPodCleanController(client, scheme, cleanupCfg)
	ctx, cancel := context.WithCancel(context.Background())

	// Run job in goroutine
	go RunPodCleanJob(ctx, controller, 100*time.Millisecond)

	time.Sleep(500 * time.Millisecond) // Let the job run at least once
	cancel()

	// Validate pod is deleted
	podList := &corev1.PodList{}
	if err := client.List(context.Background(), podList); err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}
	if len(podList.Items) != 0 {
		t.Errorf("Pod was not deleted by cleanup job: %+v", podList.Items)
	}
}

func TestPodCleanupController_PodCleanupConfigDisabled(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	// Test data: 2 pods with same label, different creation times
	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(oldPod, newPod).Build()

	cleanupCfg := &cleanupconfig.CleanupConfig{
		BatchSize: 2,
		DryRun:    false,
		PodCleanupConfig: cleanupconfig.PodCleanupConfig{
			Enabled: false,

			Rules: []cleanupconfig.PodCleanRule{
				{
					Name:    "succeeded-pods",
					Enabled: true,
					Phase:   string(corev1.PodSucceeded),
					TTL:     cleanupconfig.Duration{Duration: time.Hour},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Namespaces: []string{"default"},
				},
			},
		},
	}

	controller := NewPodCleanController(client, scheme, cleanupCfg)
	ctx := context.Background()

	// Run cleanup
	controller.RunCleanUp(ctx)

	// Validate that oldPod is deleted, newPod remains
	podList := &corev1.PodList{}
	if err := client.List(ctx, podList); err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(podList.Items) != 2 {
		t.Errorf("Unexpected pods after cleanup: %+v", podList.Items)
	}
}

func TestPodCleanupController_InvalidSelector(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	// Test data: 2 pods with same label, different creation times
	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(oldPod, newPod).Build()

	cleanupCfg := &cleanupconfig.CleanupConfig{
		BatchSize: 2,
		DryRun:    false,
		PodCleanupConfig: cleanupconfig.PodCleanupConfig{
			Enabled: false,

			Rules: []cleanupconfig.PodCleanRule{
				{
					Name:    "succeeded-pods",
					Enabled: true,
					Phase:   string(corev1.PodSucceeded),
					TTL:     cleanupconfig.Duration{Duration: time.Hour},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "[23],{vld,vld}",
						},
					},
					Namespaces: []string{"default"},
				},
			},
		},
	}

	controller := NewPodCleanController(client, scheme, cleanupCfg)
	ctx := context.Background()

	// Run cleanup
	controller.RunCleanUp(ctx)

	// Validate that oldPod is deleted, newPod remains
	podList := &corev1.PodList{}
	if err := client.List(ctx, podList); err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(podList.Items) != 2 {
		t.Errorf("Unexpected pods after cleanup: %+v", podList.Items)
	}
}
