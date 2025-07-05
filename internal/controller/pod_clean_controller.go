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

func (c *PodCleanController) RunCleanUp(ctx context.Context) {
	if !c.CleanupConfig.PodCleanupConfig.Enabled {
		return
	}

	logger := log.FromContext(ctx)
	logger.Info("Starting pod cleanup")

	for _, rule := range c.CleanupConfig.PodCleanupConfig.Rules {
		if !rule.Enabled {
			continue
		}

		logger.Info("Processing cleanup rule", "rule", rule.Name)

		pods, err := c.PodMatcher.FindPodsToCleanup(ctx, rule)
		if err != nil {
			logger.Error(err, "Failed to find pods", "rule", rule.Name)
			continue
		}

		if len(pods) == 0 {
			logger.V(1).Info("No pods to cleanup for rule", "rule", rule.Name)
			continue
		}

		logger.Info("Found pods to cleanup", "rule", rule.Name, "count", len(pods))

		if err := BatchDeletePods(ctx, c.Client, pods, c.CleanupConfig.BatchSize, c.CleanupConfig.DryRun); err != nil {
			logger.Error(err, "Failed to batch delete pods", "rule", rule.Name)
			continue
		}

		logger.Info("Completed cleanup for rule", "rule", rule.Name, "processed", len(pods))
	}

	logger.Info("Pod cleanup completed")
}

func (pm *PodMatcher) FindPodsToCleanup(ctx context.Context, rule cleanupconfig.PodCleanRule) ([]corev1.Pod, error) {
	logger := log.FromContext(ctx)
	selector, err := metav1.LabelSelectorAsSelector(&rule.Selector)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector: %w", err)
	}

	namespaces := rule.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""} // All namespaces
	}

	var podsToCleanup []corev1.Pod

	for _, namespace := range namespaces {
		var podList corev1.PodList
		if err := pm.client.List(ctx, &podList, &client.ListOptions{
			Namespace:     namespace,
			LabelSelector: selector,
		}); err != nil {
			logger.Error(err, "Failed to list pods", "namespace", namespace)
			continue
		}

		for i := range podList.Items {
			pod := &podList.Items[i]
			if pm.ShouldCleanupPod(pod, rule) {
				podsToCleanup = append(podsToCleanup, *pod)
			}
		}
	}

	return podsToCleanup, nil
}

func (pm *PodMatcher) ShouldCleanupPod(pod *corev1.Pod, rule cleanupconfig.PodCleanRule) bool {
	if string(pod.Status.Phase) != rule.Phase {
		return false
	}

	if pod.Annotations["kubeclean/disabled"] == "true" {
		return false
	}

	ttl := rule.TTL.Duration
	if ttlStr, exists := pod.Annotations["kubeclean/ttl"]; exists {
		if parsedTTL, err := time.ParseDuration(ttlStr); err == nil {
			ttl = parsedTTL
		} else {
			log.FromContext(context.TODO()).Info("Invalid TTL annotation; using rule TTL", "pod", pod.Name, "error", err)
		}
	}

	age := time.Since(pod.CreationTimestamp.Time)
	return age > ttl
}

func BatchDeletePods(ctx context.Context, k8sClient client.Client, pods []corev1.Pod, batchSize int, dryRun bool) error {
	logger := log.FromContext(ctx)

	for i := 0; i < len(pods); i += batchSize {
		end := i + batchSize
		if end > len(pods) {
			end = len(pods)
		}

		batch := pods[i:end]
		logger.Info("Processing batch", "range", fmt.Sprintf("%d-%d", i+1, end), "total", len(pods))

		for _, pod := range batch {
			if dryRun {
				logger.Info("DRY RUN: Would delete pod", "pod", pod.Name, "namespace", pod.Namespace)
				continue
			}

			logger.Info("Deleting pod", "pod", pod.Name, "namespace", pod.Namespace)
			if err := k8sClient.Delete(ctx, &pod); err != nil {
				logger.Error(err, "Failed to delete pod", "pod", pod.Name, "namespace", pod.Namespace)
			}
		}

		if end < len(pods) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

func RunPodCleanJob(ctx context.Context, controller *PodCleanController, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			controller.RunCleanUp(runCtx)
			cancel()

		case <-ctx.Done():
			return
		}
	}
}
