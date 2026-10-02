package metrics

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// ExperimentsTotal tracks the total number of chaos experiments run, labeled by type and result.
	ExperimentsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "havoc_experiments_total",
			Help: "Total number of chaos experiments executed, partitioned by type and result",
		},
		[]string{"experiment_type", "result"},
	)

	// PodRecoverySeconds tracks the time it takes for a new pod to become ready after a pod is killed.
	PodRecoverySeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "havoc_pod_recovery_seconds",
			Help:    "Time taken from pod kill to a new pod becoming Ready",
			Buckets: prometheus.DefBuckets, // Default buckets: .005 to 10.0 seconds
		},
	)

	// ExperimentErrorRate tracks the current simulated or real error rate for safety aborts.
	ExperimentErrorRate = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "havoc_experiment_error_rate",
			Help: "Current simulated or real error rate percentage used by the safety engine",
		},
	)
)

// StartMetricsServer starts an HTTP server to expose Prometheus metrics.
func StartMetricsServer(port int) {
	http.Handle("/metrics", promhttp.Handler())
	addr := fmt.Sprintf(":%d", port)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			fmt.Printf("Metrics server failed: %v\n", err)
		}
	}()
}

// PrometheusMetricsChecker implements safety.MetricsChecker using a real Prometheus client.
type PrometheusMetricsChecker struct {
	client *PromClient
}

// NewPrometheusMetricsChecker creates a new PrometheusMetricsChecker.
func NewPrometheusMetricsChecker(promBaseURL string) *PrometheusMetricsChecker {
	return &PrometheusMetricsChecker{
		client: NewPromClient(promBaseURL),
	}
}

// GetErrorRate queries the actual error rate from Prometheus and updates the Prometheus gauge.
func (c *PrometheusMetricsChecker) GetErrorRate(ctx context.Context, namespace string) (float64, error) {
	rate, err := c.client.QueryErrorRate(ctx)
	if err != nil {
		fmt.Printf("Warning: Failed to query Prometheus for error rate: %v. Falling back to 0%%.\n", err)
		rate = 0.0
	}

	// Update the gauge so Prometheus can scrape the exact value the safety engine sees.
	ExperimentErrorRate.Set(rate)

	return rate, nil
}
