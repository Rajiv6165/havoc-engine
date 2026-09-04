package postmortem

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockGenerator is a manual mock for the Generator interface.
type MockGenerator struct {
	GenerateReportFunc func(ctx context.Context, data ExperimentData) (string, error)
}

func (m *MockGenerator) GenerateReport(ctx context.Context, data ExperimentData) (string, error) {
	if m.GenerateReportFunc != nil {
		return m.GenerateReportFunc(ctx, data)
	}
	return "Mocked Report", nil
}

func TestClaudeGenerator_Success(t *testing.T) {
	// Create a test server to mock the Anthropic API
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("Expected x-api-key 'test-key', got '%s'", r.Header.Get("x-api-key"))
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"content": [
				{
					"text": "1. What happened: Test\n2. Impact: None\n3. Likely root cause: Magic\n4. Recommendation: Do nothing"
				}
			]
		}`))
	}))
	defer ts.Close()

	gen, err := NewClaudeGenerator("test-key")
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	// Override HTTP client to use our test server by wrapping the transport
	// Wait, we can't easily override the URL without changing the code, 
	// but we can just test that the template rendering works or use a custom transport.
	// Since the URL is hardcoded in generator.go, let's just use the transport mock.
	
	gen.HTTPClient = ts.Client()
	
	// We have to mock the RoundTripper to redirect the hardcoded URL to the test server.
	gen.HTTPClient.Transport = &rewriteTransport{
		TargetURL: ts.URL,
		Transport: http.DefaultTransport,
	}

	recTime := 150
	errRate := 2.5
	data := ExperimentData{
		ExperimentType:   "kill-pod",
		TargetNamespace:  "default",
		RecoveryTimeMs:   &recTime,
		ErrorRatePercent: &errRate,
		SafetyIntervened: false,
		ResilienceScore:  95,
	}

	report, err := gen.GenerateReport(context.Background(), data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(report, "What happened: Test") {
		t.Errorf("Expected report to contain 'What happened: Test', got '%s'", report)
	}
}

func TestClaudeGenerator_NoAPIKey(t *testing.T) {
	gen, err := NewClaudeGenerator("")
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	_, err = gen.GenerateReport(context.Background(), ExperimentData{})
	if err == nil || !strings.Contains(err.Error(), "API key is not set") {
		t.Errorf("Expected error about missing API key, got: %v", err)
	}
}

type rewriteTransport struct {
	TargetURL string
	Transport http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the request URL to our test server
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.TargetURL, "http://")
	return t.Transport.RoundTrip(req)
}
