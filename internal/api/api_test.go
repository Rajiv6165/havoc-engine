package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"havoc-engine/internal/config"
	"havoc-engine/internal/events"
	"havoc-engine/internal/scoring"
	"havoc-engine/internal/storage"
)

type mockRepo struct {
	experiments []storage.ExperimentResult
}

func (m *mockRepo) SaveExperimentResult(ctx context.Context, result *storage.ExperimentResult) error {
	m.experiments = append(m.experiments, *result)
	return nil
}

func (m *mockRepo) GetRecentExperiments(ctx context.Context, limit int) ([]storage.ExperimentResult, error) {
	if limit > len(m.experiments) {
		limit = len(m.experiments)
	}
	return m.experiments[:limit], nil
}

func (m *mockRepo) GetScoreHistory(ctx context.Context, limit int) ([]int, error) {
	return []int{85}, nil
}

func (m *mockRepo) LogScheduledRun(ctx context.Context, outcome, experimentFile string) error {
	return nil
}

func (m *mockRepo) GetRecentScheduledRuns(ctx context.Context, limit int) ([]storage.ScheduledRun, error) {
	return []storage.ScheduledRun{}, nil
}

func (m *mockRepo) GetSchedulerState(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *mockRepo) SetSchedulerState(ctx context.Context, paused bool) error {
	return nil
}

func int32Ptr(i int32) *int32 {
	return &i
}

func TestAPI_GetStatus(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "default"},
			Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(2)},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(2)},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
			Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(1)},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 0},
		},
	)

	cfg := &config.Config{
		DefaultNamespace: "default",
		TargetNamespaces: []string{"default"},
		Server: config.ServerConfig{
			Port:        8080,
			AllowedCORS: []string{"*"},
		},
	}

	hub := events.NewHub()
	defer hub.Close()

	srv := NewServer(cfg, client, nil, hub, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var statuses []ServiceStatus
	if err := json.NewDecoder(w.Body).Decode(&statuses); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(statuses) != 3 {
		t.Fatalf("expected 3 service statuses, got %d", len(statuses))
	}

	statusMap := make(map[string]ServiceStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s
	}

	if s, ok := statusMap["frontend"]; !ok || s.Status != "healthy" || s.PodCount != 2 {
		t.Errorf("frontend status mismatch: %+v", s)
	}
	if s, ok := statusMap["api"]; !ok || s.Status != "degraded" || s.PodCount != 1 {
		t.Errorf("api status mismatch: %+v", s)
	}
	if s, ok := statusMap["db"]; !ok || s.Status != "critical" || s.PodCount != 0 {
		t.Errorf("db status mismatch: %+v", s)
	}
}

func TestAPI_GetTopology(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "frontend",
				Namespace:   "default",
				Annotations: map[string]string{"havoc.io/depends-on": "api"},
			},
			Spec:   appsv1.DeploymentSpec{Replicas: int32Ptr(2)},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "api",
				Namespace:   "default",
				Annotations: map[string]string{"havoc.io/depends-on": "db,queue"},
			},
			Spec:   appsv1.DeploymentSpec{Replicas: int32Ptr(2)},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
			Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(1)},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "queue", Namespace: "default"},
			Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(1)},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
		},
	)

	cfg := &config.Config{
		DefaultNamespace: "default",
		TargetNamespaces: []string{"default"},
		Server:           config.ServerConfig{Port: 8080},
	}

	hub := events.NewHub()
	defer hub.Close()

	srv := NewServer(cfg, client, nil, hub, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/topology", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp TopologyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Fallback {
		t.Errorf("expected fallback to be false for real deployments")
	}

	if len(resp.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(resp.Nodes))
	}

	if len(resp.Edges) != 3 {
		t.Errorf("expected 3 edges (frontend->api, api->db, api->queue), got %d", len(resp.Edges))
	}
}

func TestAPI_GetTopologyFallback(t *testing.T) {
	// Empty clientset triggers fallback
	client := fake.NewSimpleClientset()

	cfg := &config.Config{
		DefaultNamespace: "default",
		TargetNamespaces: []string{"default"},
		Server:           config.ServerConfig{Port: 8080},
		Topology: config.TopologyConfig{
			Nodes: []config.TopologyNode{
				{ID: "svc-a", Label: "svc-a", PodCount: 2},
				{ID: "svc-b", Label: "svc-b", PodCount: 1},
			},
			Edges: []config.TopologyEdge{
				{Source: "svc-a", Target: "svc-b"},
			},
		},
	}

	srv := NewServer(cfg, client, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/topology", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp TopologyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Fallback {
		t.Errorf("expected fallback to be true")
	}

	if len(resp.Nodes) != 2 {
		t.Errorf("expected 2 fallback nodes from config, got %d", len(resp.Nodes))
	}
}

func TestAPI_PostExperiments(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "frontend-xyz",
				Namespace: "default",
				Labels:    map[string]string{"app": "frontend"},
			},
		},
	)

	cfg := &config.Config{
		DefaultNamespace:      "default",
		MaxBlastRadiusPercent: 100.0,
		Scoring:               scoring.DefaultConfig(),
		Server:                config.ServerConfig{Port: 8080},
	}

	hub := events.NewHub()
	defer hub.Close()

	srv := NewServer(cfg, client, nil, hub, nil)

	payload := ExperimentRequest{
		Action:    "kill-pod",
		Namespace: "default",
		Selector:  "app=frontend",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/experiments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "accepted" {
		t.Errorf("expected status accepted, got %v", resp["status"])
	}

	idStr, ok := resp["id"].(string)
	if !ok || idStr == "" {
		t.Errorf("expected valid id string in response")
	}
}

func TestAPI_GetExperiments(t *testing.T) {
	sampleReport := "AI postmortem report content"
	recoveryTime := 1200
	errRate := 0.5
	repo := &mockRepo{
		experiments: []storage.ExperimentResult{
			{
				ID:               uuid.New(),
				ExperimentType:   "kill-pod",
				TargetNamespace:  "default",
				RecoveryTimeMs:   &recoveryTime,
				ErrorRatePercent: &errRate,
				SafetyIntervened: false,
				ResilienceScore:  90,
				PostmortemReport: &sampleReport,
				CreatedAt:        time.Now(),
			},
		},
	}

	cfg := &config.Config{
		DefaultNamespace: "default",
		Server:           config.ServerConfig{Port: 8080},
	}

	srv := NewServer(cfg, nil, repo, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/experiments?limit=5", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var results []storage.ExperimentResult
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 experiment result, got %d", len(results))
	}

	if results[0].ResilienceScore != 90 {
		t.Errorf("expected score 90, got %d", results[0].ResilienceScore)
	}
	if results[0].PostmortemReport == nil || *results[0].PostmortemReport != sampleReport {
		t.Errorf("expected postmortem report %q, got %v", sampleReport, results[0].PostmortemReport)
	}
}

func TestAPI_CORSMiddleware(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			AllowedCORS: []string{"http://localhost:3000"},
		},
	}

	srv := NewServer(cfg, nil, nil, nil, nil)

	// Preflight OPTIONS request
	optReq := httptest.NewRequest(http.MethodOptions, "/api/status", nil)
	optReq.Header.Set("Origin", "http://localhost:3000")
	optW := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(optW, optReq)

	if optW.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content for OPTIONS, got %d", optW.Code)
	}
	if optW.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected CORS origin header http://localhost:3000, got %q", optW.Header().Get("Access-Control-Allow-Origin"))
	}

	// GET request with Origin
	getReq := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	getReq.Header.Set("Origin", "http://localhost:3000")
	getW := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(getW, getReq)

	if getW.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected CORS header on GET, got %q", getW.Header().Get("Access-Control-Allow-Origin"))
	}
}
