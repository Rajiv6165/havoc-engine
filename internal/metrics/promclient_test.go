package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryErrorRate_NormalCase(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := PromQueryResponse{
			Status: "success",
		}
		// Structure result matching normal prom response
		resp.Data.ResultType = "vector"
		
		// For normal case, let's say the error rate is 25.5
		result := struct {
			Value []interface{} `json:"value"`
		}{
			Value: []interface{}{1616161616.0, "25.5"},
		}
		resp.Data.Result = append(resp.Data.Result, result)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewPromClient(ts.URL)
	rate, err := client.QueryErrorRate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rate != 25.5 {
		t.Errorf("expected rate 25.5, got %v", rate)
	}
}

func TestQueryErrorRate_EmptyCase(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := PromQueryResponse{
			Status: "success",
		}
		// Return empty results (e.g. no data matched the query)
		resp.Data.ResultType = "vector"
		resp.Data.Result = nil
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewPromClient(ts.URL)
	rate, err := client.QueryErrorRate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rate != 0.0 {
		t.Errorf("expected rate 0.0, got %v", rate)
	}
}

func TestQueryErrorRate_NaNCase(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := PromQueryResponse{
			Status: "success",
		}
		resp.Data.ResultType = "vector"
		
		// When denominator is 0, prom might return NaN as a string
		result := struct {
			Value []interface{} `json:"value"`
		}{
			Value: []interface{}{1616161616.0, "NaN"},
		}
		resp.Data.Result = append(resp.Data.Result, result)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewPromClient(ts.URL)
	rate, err := client.QueryErrorRate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rate != 0.0 {
		t.Errorf("expected rate 0.0, got %v", rate)
	}
}

func TestQueryErrorRate_Unreachable(t *testing.T) {
	// Creating a client to a server that doesn't exist
	client := NewPromClient("http://127.0.0.1:0") // Guaranteed connection refused
	_, err := client.QueryErrorRate(context.Background())
	if err == nil {
		t.Fatalf("expected connection error, got nil")
	}
}

func TestPrometheusMetricsChecker_UnreachableFallback(t *testing.T) {
	// The safety runner uses PrometheusMetricsChecker, which should fallback to 0 instead of crashing.
	checker := NewPrometheusMetricsChecker("http://127.0.0.1:0")
	rate, err := checker.GetErrorRate(context.Background(), "default")
	if err != nil {
		t.Fatalf("expected no error from checker, got: %v", err)
	}
	if rate != 0.0 {
		t.Errorf("expected rate 0.0 fallback, got %v", rate)
	}
}
