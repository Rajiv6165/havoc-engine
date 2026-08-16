package safety

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"havoc-engine/internal/metrics"
)

func TestMetricsIncrementAfterExperiment(t *testing.T) {
	// Reset the counter for testing
	metrics.ExperimentsTotal.Reset()

	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "demo",
			},
		},
	})

	checker := metrics.NewPrometheusMetricsChecker(1.0)
	runner := NewSafetyRunner(client, 100.0, 10.0, false, checker)

	ctx := context.Background()
	_, err := runner.ExecuteKillPod(ctx, "default", "app=demo")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify that the success metric was incremented
	count := testutil.ToFloat64(metrics.ExperimentsTotal.WithLabelValues("kill_pod", "success"))
	if count != 1 {
		t.Errorf("expected metric count 1, got %v", count)
	}
}
