package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PromClient is a minimalistic HTTP client for querying Prometheus.
type PromClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewPromClient creates a new minimal Prometheus client.
func NewPromClient(baseURL string) *PromClient {
	return &PromClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// PromQueryResponse represents the minimal Prometheus API response we need.
type PromQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// QueryErrorRate queries Prometheus for the recent error rate percentage.
func (c *PromClient) QueryErrorRate(ctx context.Context) (float64, error) {
	// Formula: (failed + aborted) / total over last 15 minutes.
	query := `sum(increase(havoc_experiments_total{result=~"failed|aborted"}[15m])) / sum(increase(havoc_experiments_total[15m])) * 100`

	reqURL := fmt.Sprintf("%s/api/v1/query?query=%s", c.BaseURL, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	var result PromQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "success" {
		return 0, fmt.Errorf("prometheus query failed: %s", result.Status)
	}

	if len(result.Data.Result) == 0 {
		// No data, which means no experiments have been run yet.
		return 0, nil
	}

	valueArr := result.Data.Result[0].Value
	if len(valueArr) < 2 {
		return 0, fmt.Errorf("unexpected value format from prometheus")
	}

	valStr, ok := valueArr[1].(string)
	if !ok {
		return 0, fmt.Errorf("failed to parse prometheus value as string")
	}

	rate, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse rate to float: %w", err)
	}

	// Handle NaN when denominator is 0 (i.e. increase is 0 for both numerator and denominator)
	if math.IsNaN(rate) {
		return 0, nil
	}

	return rate, nil
}
