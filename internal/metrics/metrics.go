package metrics

import (
	"context"
	"fmt"
	"math/rand"
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

// PrometheusMetricsChecker implements safety.MetricsChecker.
type PrometheusMetricsChecker struct {
	// Base rate for simulated errors.
	BaseRate float64
}

// NewPrometheusMetricsChecker creates a new PrometheusMetricsChecker.
func NewPrometheusMetricsChecker(baseRate float64) *PrometheusMetricsChecker {
	return &PrometheusMetricsChecker{
		BaseRate: baseRate,
	}
}

// GetErrorRate returns a simulated error rate and updates the Prometheus gauge.
func (c *PrometheusMetricsChecker) GetErrorRate(ctx context.Context, namespace string) (float64, error) {
	// Simulate an error rate around the BaseRate.
	variation := (rand.Float64() * 4.0) - 2.0
	currentRate := c.BaseRate + variation
	if currentRate < 0 {
		currentRate = 0
	}

	// Update the gauge so Prometheus can scrape the exact value the safety engine sees.
	ExperimentErrorRate.Set(currentRate)

	return currentRate, nil
}
