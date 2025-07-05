package controller

import (
	"context"
	"fmt"
	"time"

	cleanupconfig "github.com/infrautils/kubeclean/internal/cleanup_config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PodCleanController struct {
	Client        client.Client
	Scheme        *runtime.Scheme
	CleanupConfig *cleanupconfig.CleanupConfig
	PodMatcher    *PodMatcher
}

func NewPodCleanController(k8sClient client.Client, scheme *runtime.Scheme, cleanupConfig *cleanupconfig.CleanupConfig) *PodCleanController {
	return &PodCleanController{
		Client:        k8sClient,
		Scheme:        scheme,
		CleanupConfig: cleanupConfig,
		PodMatcher:    NewPodMatcher(k8sClient),
	}
}

type PodMatcher struct {
	client client.Client
}

func NewPodMatcher(k8sClient client.Client) *PodMatcher {
	return &PodMatcher{client: k8sClient}
}

func (r *PodCleanController) runCleanUp(ctx context.Context) {
	if !r.CleanupConfig.PodCleanupConfig.Enabled {
		return
	}

	logger := log.FromContext(ctx)

	logger.Info("Starting batch cleanup of pods")

	for _, rule := range r.CleanupConfig.PodCleanupConfig.Rules {
		if !rule.Enabled {
			continue
		}

		logger.Info("Processing cleanup rule", "rule", rule.Name)
		pods, err := r.PodMatcher.findPodsToCleanup(ctx, rule)
		if err != nil {
			logger.Error(err, "Failed to find pods for cleanup", "rule", rule.Name)
			continue
		}

		if len(pods) == 0 {
			logger.V(1).Info("No pods to cleanup for rule", "rule", rule.Name)
			continue
		}

		logger.Info("Found pods to cleanup", "rule", rule.Name, "count", len(pods))
		if failed := batchDeletePods(ctx, r.Client, pods, r.CleanupConfig.BatchSize, r.CleanupConfig.DryRun); failed {
			logger.Error(fmt.Errorf("failed to batch delete pods"), "rule", rule.Name)
			continue
		}

		logger.Info("Completed cleanup for rule", "rule", rule.Name, "processed", len(pods))
	}

	logger.Info("Ending batch cleanup of pods")

}

func (pm *PodMatcher) findPodsToCleanup(ctx context.Context, rule cleanupconfig.PodCleanRule) ([]corev1.Pod, error) {
	logger := log.FromContext(ctx)
	var podsToCleanup []corev1.Pod

	selector, err := metav1.LabelSelectorAsSelector(&rule.Selector)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector: %w", err)
	}

	namespaces := rule.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""} // All namespaces
	}

	for _, namespace := range namespaces {
		var podList corev1.PodList
		opts := &client.ListOptions{
			Namespace:     namespace,
			LabelSelector: selector,
		}

		if err := pm.client.List(ctx, &podList, opts); err != nil {
			logger.Error(err, "Failed to list pods", "namespace", namespace)
			continue
		}

		for i := range podList.Items {
			pod := &podList.Items[i]
			if pm.shouldCleanupPod(pod, rule) {
				podsToCleanup = append(podsToCleanup, *pod)
			}
		}
	}

	return podsToCleanup, nil
}

func (pm *PodMatcher) shouldCleanupPod(pod *corev1.Pod, rule cleanupconfig.PodCleanRule) bool {
	if string(pod.Status.Phase) != rule.Phase {
		return false
	}

	if disabled, exists := pod.Annotations["kubeclean/disabled"]; exists && disabled == "true" {
		return false
	}

	ttl := rule.TTL.Duration
	if ttlStr, exists := pod.Annotations["kubeclean/ttl"]; exists {
		if parsedTTL, err := time.ParseDuration(ttlStr); err == nil {
			ttl = parsedTTL
		} else {
			log.FromContext(context.TODO()).Info("Invalid TTL annotation, using rule TTL", "pod", pod.Name, "error", err)
		}
	}

	age := time.Since(pod.CreationTimestamp.Time)
	return age > ttl
}

func batchDeletePods(ctx context.Context, k8sClient client.Client, pods []corev1.Pod, batchSize int, dryRun bool) bool {
	logger := log.FromContext(ctx)

	var anyFailed bool

	for i := 0; i < len(pods); i += batchSize {
		end := i + batchSize
		if end > len(pods) {
			end = len(pods)
		}

		batch := pods[i:end]
		logger.Info("Processing batch", "range", fmt.Sprintf("%d-%d", i+1, end), "total", len(pods))

		for _, pod := range batch {
			if dryRun {
				logger.Info("DRY RUN: Would delete pod", "pod", pod.Name, "namespace", pod.Namespace, "age", time.Since(pod.CreationTimestamp.Time), "phase", pod.Status.Phase)
				continue
			}

			logger.Info("Deleting pod", "pod", pod.Name, "namespace", pod.Namespace, "age", time.Since(pod.CreationTimestamp.Time))
			if err := k8sClient.Delete(ctx, &pod); err != nil {
				logger.Error(err, "Failed to delete pod", "pod", pod.Name, "namespace", pod.Namespace)

				anyFailed = true
				continue
			}
		}

		// Sleep between batches to avoid API server overload
		if end < len(pods) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return anyFailed
}

func RunPodCleanJob(ctx context.Context, podCleanController *PodCleanController, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)

			defer cancel()

			podCleanController.runCleanUp(runCtx)

		case <-ctx.Done():
			return
		}
	}

}
