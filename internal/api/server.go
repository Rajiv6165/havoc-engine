package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"havoc-engine/internal/config"
	"havoc-engine/internal/events"
	"havoc-engine/internal/postmortem"
	"havoc-engine/internal/safety"
	"havoc-engine/internal/scoring"
	"havoc-engine/internal/storage"
)

// RateLimiter tracks client requests within a rolling time window.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new RateLimiter instance.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow returns true if the client IP has not exceeded the rate limit.
func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	var recent []time.Time
	for _, t := range rl.requests[clientIP] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= rl.limit {
		rl.requests[clientIP] = recent
		return false
	}

	recent = append(recent, now)
	rl.requests[clientIP] = recent
	return true
}

// Server provides HTTP API endpoints for havoc-engine.
type Server struct {
	httpServer     *http.Server
	cfg            *config.Config
	k8sClient      kubernetes.Interface
	repo           storage.ExperimentRepository
	hub            *events.Hub
	metricsChecker safety.MetricsChecker

	rateLimiter   *RateLimiter
	activeMu      sync.Mutex
	activeTargets map[string]bool
}

// NewServer initializes a new API server.
func NewServer(
	cfg *config.Config,
	client kubernetes.Interface,
	repo storage.ExperimentRepository,
	hub *events.Hub,
	checker safety.MetricsChecker,
) *Server {
	s := &Server{
		cfg:            cfg,
		k8sClient:      client,
		repo:           repo,
		hub:            hub,
		metricsChecker: checker,
		rateLimiter:    NewRateLimiter(10, time.Minute),
		activeTargets:  make(map[string]bool),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleGetStatus)
	mux.HandleFunc("POST /api/experiments", s.handlePostExperiments)
	mux.HandleFunc("GET /api/experiments", s.handleGetExperiments)
	mux.HandleFunc("GET /api/topology", s.handleGetTopology)

	corsHandler := CORSMiddleware(cfg.Server.AllowedCORS)(mux)

	host := cfg.Server.Host
	if host == "" {
		host = cfg.Server.BindAddress
	}
	if host == "" {
		host = "127.0.0.1"
	}

	port := cfg.Server.Port
	if port <= 0 {
		port = 8080
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      corsHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return s
}

func (s *Server) authenticate(r *http.Request) bool {
	if s.cfg == nil || s.cfg.API.Token == "" {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	return token == s.cfg.API.Token
}

func (s *Server) checkReadAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg != nil && s.cfg.API.Token != "" && s.cfg.API.RequireAuthForReads {
		if !s.authenticate(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized: missing or invalid bearer token"})
			return false
		}
	}
	return true
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// CORSMiddleware provides configurable CORS handling for the dashboard origin.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false

			if len(allowedOrigins) == 0 {
				allowed = true
			} else {
				for _, o := range allowedOrigins {
					if o == "*" || o == origin {
						allowed = true
						break
					}
				}
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else if len(allowedOrigins) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ServiceStatus represents deployment health for the status endpoint.
type ServiceStatus struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	PodCount        int    `json:"podCount"`
	ReadyReplicas   int32  `json:"readyReplicas"`
	DesiredReplicas int32  `json:"desiredReplicas"`
	Status          string `json:"status"` // "healthy", "degraded", "critical"
}

// handleGetStatus lists Deployments in configured namespaces and reports health state.
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if !s.checkReadAuth(w, r) {
		return
	}
	var statuses []ServiceStatus

	if s.k8sClient != nil {
		namespaces := s.cfg.TargetNamespaces
		if len(namespaces) == 0 {
			namespaces = []string{s.cfg.DefaultNamespace}
		}

		for _, ns := range namespaces {
			deployments, err := s.k8sClient.AppsV1().Deployments(ns).List(r.Context(), metav1.ListOptions{})
			if err != nil {
				continue
			}

			for _, d := range deployments.Items {
				desired := int32(1)
				if d.Spec.Replicas != nil {
					desired = *d.Spec.Replicas
				}
				ready := d.Status.ReadyReplicas

				health := "healthy"
				if desired > 0 && ready == 0 {
					health = "critical"
				} else if ready < desired {
					health = "degraded"
				}

				statuses = append(statuses, ServiceStatus{
					Name:            d.Name,
					Namespace:       ns,
					PodCount:        int(ready),
					ReadyReplicas:   ready,
					DesiredReplicas: desired,
					Status:          health,
				})
			}
		}
	}

	if statuses == nil {
		statuses = []ServiceStatus{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statuses)
}

// ExperimentRequest is the payload accepted by POST /api/experiments.
type ExperimentRequest struct {
	Action    string            `json:"action"` // "kill-pod", "inject-latency", "spike-cpu"
	Namespace string            `json:"namespace"`
	Selector  string            `json:"selector"`
	Pod       string            `json:"pod"`
	Params    ExperimentParams  `json:"params"`
}

type ExperimentParams struct {
	DelayMs     int `json:"delayMs"`
	DurationSec int `json:"durationSec"`
}

// handlePostExperiments accepts an experiment request, starts it in a goroutine, and returns 202.
func (s *Server) handlePostExperiments(w http.ResponseWriter, r *http.Request) {
	if s.cfg != nil && s.cfg.API.Token != "" && !s.authenticate(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized: missing or invalid bearer token"})
		return
	}

	clientIP := getClientIP(r)
	if s.rateLimiter != nil && !s.rateLimiter.Allow(clientIP) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded: max 10 requests per minute"})
		return
	}

	var req ExperimentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json body"})
		return
	}

	if req.Action == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "action is required"})
		return
	}

	if req.Namespace == "" {
		req.Namespace = s.cfg.DefaultNamespace
	}

	// Auto-discover target if none provided (e.g. from UI buttons)
	if req.Selector == "" && req.Pod == "" && s.k8sClient != nil {
		if deps, err := s.k8sClient.AppsV1().Deployments(req.Namespace).List(r.Context(), metav1.ListOptions{}); err == nil && len(deps.Items) > 0 {
			req.Selector = fmt.Sprintf("app=%s", deps.Items[0].Name)
		} else if pods, err := s.k8sClient.CoreV1().Pods(req.Namespace).List(r.Context(), metav1.ListOptions{}); err == nil && len(pods.Items) > 0 {
			req.Pod = pods.Items[0].Name
			if len(pods.Items[0].Labels) > 0 {
				if app, ok := pods.Items[0].Labels["app"]; ok {
					req.Selector = fmt.Sprintf("app=%s", app)
				}
			}
		}
	}

	target := req.Selector
	if target == "" {
		target = req.Pod
	}
	if target == "" {
		target = "target"
	}
	targetKey := req.Namespace + "/" + target

	s.activeMu.Lock()
	if s.activeTargets[targetKey] {
		s.activeMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("an active experiment is already running on target %q in namespace %q", target, req.Namespace),
		})
		return
	}
	s.activeTargets[targetKey] = true
	s.activeMu.Unlock()

	expID := uuid.New()

	// Launch experiment asynchronously
	go s.runExperiment(expID, req, targetKey)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      expID.String(),
		"status":  "accepted",
		"message": fmt.Sprintf("Experiment %s (%s) accepted", expID, req.Action),
	})
}

func (s *Server) runExperiment(expID uuid.UUID, req ExperimentRequest, targetKey string) {
	defer func() {
		s.activeMu.Lock()
		delete(s.activeTargets, targetKey)
		s.activeMu.Unlock()
	}()

	service := req.Selector
	if idx := strings.Index(service, "="); idx != -1 {
		service = service[idx+1:]
	}
	if service == "" {
		service = req.Pod
	}
	if service == "" {
		service = "target"
	}

	// Publish experiment_started event
	if s.hub != nil {
		s.hub.Publish(events.Event{
			ExperimentID: expID.String(),
			Type:         events.EventExperimentStarted,
			Level:        "info",
			Message:      fmt.Sprintf("[%s] experiment started: %s on service %q", expID, req.Action, service),
			Service:      service,
		})
	}

	runner := safety.NewSafetyRunner(s.k8sClient, s.cfg.MaxBlastRadiusPercent, s.cfg.AbortOnErrorRatePercent, false, s.metricsChecker)
	if s.hub != nil {
		runner.SetPublisher(s.hub)
	}

	ctx := safety.WithExperimentContext(context.Background(), expID.String(), service)

	var (
		deletedPod     string
		recoveryTimeMs *int
		expErr         error
	)

	switch req.Action {
	case "kill-pod":
		selector := req.Selector
		if selector == "" && req.Pod != "" {
			// Find pod label selector
			if s.k8sClient != nil {
				if podObj, err := s.k8sClient.CoreV1().Pods(req.Namespace).Get(ctx, req.Pod, metav1.GetOptions{}); err == nil {
					if app, ok := podObj.Labels["app"]; ok {
						selector = fmt.Sprintf("app=%s", app)
					}
				}
			}
		}
		deletedPod, recoveryTimeMs, expErr = runner.ExecuteKillPod(ctx, req.Namespace, selector)
		_ = deletedPod

	case "inject-latency":
		delay := req.Params.DelayMs
		if delay <= 0 {
			delay = 500
		}
		targetPod := req.Pod
		if targetPod == "" && s.k8sClient != nil {
			if pods, err := s.k8sClient.CoreV1().Pods(req.Namespace).List(ctx, metav1.ListOptions{LabelSelector: req.Selector}); err == nil && len(pods.Items) > 0 {
				targetPod = pods.Items[0].Name
			}
		}
		recoveryTimeMs, expErr = runner.ExecuteInjectLatency(ctx, req.Namespace, targetPod, delay)

	case "spike-cpu":
		duration := req.Params.DurationSec
		if duration <= 0 {
			duration = 30
		}
		targetPod := req.Pod
		if targetPod == "" && s.k8sClient != nil {
			if pods, err := s.k8sClient.CoreV1().Pods(req.Namespace).List(ctx, metav1.ListOptions{LabelSelector: req.Selector}); err == nil && len(pods.Items) > 0 {
				targetPod = pods.Items[0].Name
			}
		}
		recoveryTimeMs, expErr = runner.ExecuteSpikeCPU(ctx, req.Namespace, targetPod, duration)
	}

	// Safety intervention detection
	safetyIntervened := false
	if expErr != nil && strings.Contains(expErr.Error(), "auto-abort triggered") {
		safetyIntervened = true
	}

	// Metrics error rate
	var errRate *float64
	if s.metricsChecker != nil {
		if rate, err := s.metricsChecker.GetErrorRate(ctx, req.Namespace); err == nil {
			errRate = &rate
		}
	}

	var errorRateVal float64
	if errRate != nil {
		errorRateVal = *errRate
	}

	score := scoring.CalculateResilienceScore(recoveryTimeMs, errorRateVal, safetyIntervened, s.cfg.Scoring)

	// Postmortem report generation
	var reportTextPtr *string
	if s.cfg.AnthropicAPIKey != "" {
		if gen, err := postmortem.NewClaudeGenerator(s.cfg.AnthropicAPIKey); err == nil {
			pmCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if report, err := gen.GenerateReport(pmCtx, postmortem.ExperimentData{
				ExperimentType:   req.Action,
				TargetNamespace:  req.Namespace,
				RecoveryTimeMs:   recoveryTimeMs,
				ErrorRatePercent: errRate,
				SafetyIntervened: safetyIntervened,
				ResilienceScore:  score,
			}); err == nil {
				reportTextPtr = &report
			}
		}
	}

	// Storage persistence
	if s.repo != nil {
		res := &storage.ExperimentResult{
			ID:               expID,
			ExperimentType:   req.Action,
			TargetNamespace:  req.Namespace,
			RecoveryTimeMs:   recoveryTimeMs,
			ErrorRatePercent: errRate,
			SafetyIntervened: safetyIntervened,
			ResilienceScore:  score,
			PostmortemReport: reportTextPtr,
			CreatedAt:        time.Now(),
		}
		_ = s.repo.SaveExperimentResult(ctx, res)
	}

	// Publish completed event with score
	if s.hub != nil {
		s.hub.Publish(events.Event{
			ExperimentID: expID.String(),
			Type:         events.EventCompleted,
			Level:        "info",
			Message:      fmt.Sprintf("[%s] experiment ended: completed with resilience score %d/100 (resolved)", expID, score),
			Service:      service,
			Score:        &score,
			RecoveryMs:   recoveryTimeMs,
		})
	}
}

// handleGetExperiments retrieves recent experiments from the database.
func (s *Server) handleGetExperiments(w http.ResponseWriter, r *http.Request) {
	if !s.checkReadAuth(w, r) {
		return
	}
	limit := 20
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 100 {
		limit = 100
	}

	if s.repo != nil {
		results, err := s.repo.GetRecentExperiments(r.Context(), limit)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(results)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode([]interface{}{})
}

// TopologyResponse represents the service graph payload for the frontend.
type TopologyResponse struct {
	Fallback bool           `json:"fallback"`
	Nodes    []TopologyNode `json:"nodes"`
	Edges    []TopologyEdge `json:"edges"`
}

type TopologyNode struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Position Position         `json:"position"`
	Data     TopologyNodeData `json:"data"`
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type TopologyNodeData struct {
	Label    string `json:"label"`
	PodCount int    `json:"podCount"`
	Status   string `json:"status"`
}

type TopologyEdge struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Animated bool   `json:"animated"`
}

// handleGetTopology builds the service dependency graph from deployment annotations or fallback.
func (s *Server) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	if !s.checkReadAuth(w, r) {
		return
	}
	resp := s.buildTopology(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) buildTopology(ctx context.Context) TopologyResponse {
	if s.k8sClient != nil {
		namespaces := s.cfg.TargetNamespaces
		if len(namespaces) == 0 {
			namespaces = []string{s.cfg.DefaultNamespace}
		}

		type serviceInfo struct {
			name      string
			podCount  int
			dependsOn []string
		}

		var services []serviceInfo
		for _, ns := range namespaces {
			deps, err := s.k8sClient.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}

			for _, d := range deps.Items {
				var depends []string
				// Check annotations first, then labels
				if d.Annotations != nil {
					if depStr, ok := d.Annotations["havoc.io/depends-on"]; ok && depStr != "" {
						for _, p := range strings.Split(depStr, ",") {
							if trimmed := strings.TrimSpace(p); trimmed != "" {
								depends = append(depends, trimmed)
							}
						}
					}
				}
				if len(depends) == 0 && d.Labels != nil {
					if depStr, ok := d.Labels["havoc.io/depends-on"]; ok && depStr != "" {
						for _, p := range strings.Split(depStr, ",") {
							if trimmed := strings.TrimSpace(p); trimmed != "" {
								depends = append(depends, trimmed)
							}
						}
					}
				}

				services = append(services, serviceInfo{
					name:      d.Name,
					podCount:  int(d.Status.ReadyReplicas),
					dependsOn: depends,
				})
			}
		}

		if len(services) > 0 {
			// Calculate dependencies and positions
			incomingCount := make(map[string]int)
			for _, svc := range services {
				for _, dep := range svc.dependsOn {
					incomingCount[dep]++
				}
			}

			// Group into layers: root (no incoming), middle, leaves
			var row0, row1, row2 []serviceInfo
			for _, svc := range services {
				if incomingCount[svc.name] == 0 {
					row0 = append(row0, svc)
				} else if len(svc.dependsOn) == 0 {
					row2 = append(row2, svc)
				} else {
					row1 = append(row1, svc)
				}
			}

			if len(row0) == 0 {
				row0 = services
			}

			var nodes []TopologyNode
			var edges []TopologyEdge
			edgeIdx := 1

			layoutRow := func(row []serviceInfo, y int) {
				count := len(row)
				for i, svc := range row {
					x := 100 + i*220
					if count == 1 {
						x = 300
					}
					nodes = append(nodes, TopologyNode{
						ID:       svc.name,
						Type:     "serviceNode",
						Position: Position{X: x, Y: y},
						Data: TopologyNodeData{
							Label:    svc.name,
							PodCount: svc.podCount,
							Status:   "healthy",
						},
					})
					for _, dep := range svc.dependsOn {
						edges = append(edges, TopologyEdge{
							ID:       fmt.Sprintf("e%d", edgeIdx),
							Source:   svc.name,
							Target:   dep,
							Animated: false,
						})
						edgeIdx++
					}
				}
			}

			layoutRow(row0, 50)
			if len(row1) > 0 {
				layoutRow(row1, 200)
			}
			if len(row2) > 0 {
				layoutRow(row2, 350)
			}

			return TopologyResponse{
				Fallback: false,
				Nodes:    nodes,
				Edges:    edges,
			}
		}
	}

	// Fallback to config topology
	if len(s.cfg.Topology.Nodes) > 0 {
		var nodes []TopologyNode
		var edges []TopologyEdge
		for i, n := range s.cfg.Topology.Nodes {
			x := 100 + (i%3)*200
			y := 50 + (i/3)*150
			nodes = append(nodes, TopologyNode{
				ID:       n.ID,
				Type:     "serviceNode",
				Position: Position{X: x, Y: y},
				Data: TopologyNodeData{
					Label:    n.Label,
					PodCount: n.PodCount,
					Status:   "healthy",
				},
			})
		}
		for i, e := range s.cfg.Topology.Edges {
			edges = append(edges, TopologyEdge{
				ID:       fmt.Sprintf("e%d", i+1),
				Source:   e.Source,
				Target:   e.Target,
				Animated: false,
			})
		}
		return TopologyResponse{
			Fallback: true,
			Nodes:    nodes,
			Edges:    edges,
		}
	}

	// Default fallback topology
	return TopologyResponse{
		Fallback: true,
		Nodes: []TopologyNode{
			{ID: "api-gateway", Type: "serviceNode", Position: Position{X: 300, Y: 50}, Data: TopologyNodeData{Label: "api-gateway", PodCount: 3, Status: "healthy"}},
			{ID: "payment-service", Type: "serviceNode", Position: Position{X: 100, Y: 200}, Data: TopologyNodeData{Label: "payment-service", PodCount: 2, Status: "healthy"}},
			{ID: "user-service", Type: "serviceNode", Position: Position{X: 500, Y: 200}, Data: TopologyNodeData{Label: "user-service", PodCount: 4, Status: "healthy"}},
			{ID: "database-service", Type: "serviceNode", Position: Position{X: 100, Y: 350}, Data: TopologyNodeData{Label: "database-service", PodCount: 1, Status: "healthy"}},
			{ID: "queue-service", Type: "serviceNode", Position: Position{X: 300, Y: 350}, Data: TopologyNodeData{Label: "queue-service", PodCount: 2, Status: "healthy"}},
		},
		Edges: []TopologyEdge{
			{ID: "e1", Source: "api-gateway", Target: "payment-service", Animated: false},
			{ID: "e2", Source: "api-gateway", Target: "user-service", Animated: false},
			{ID: "e3", Source: "payment-service", Target: "database-service", Animated: false},
			{ID: "e4", Source: "payment-service", Target: "queue-service", Animated: false},
			{ID: "e5", Source: "user-service", Target: "queue-service", Animated: false},
		},
	}
}
