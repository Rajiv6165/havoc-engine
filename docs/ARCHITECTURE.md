# Havoc Engine Architecture & Package Layout

This document provides a concise architectural overview of `havoc-engine`, detailing package responsibilities across `internal/*` and core design decisions.

---

## 🏛 Package Layout (`internal/*`)

Each package in `internal/` has a focused, single responsibility:

| Package | Primary Responsibility |
|---|---|
| [`internal/api`](file:///internal/api) | **HTTP REST API Server**: Exposes `/api/status`, `/api/experiments`, and `/api/topology`. Manages Bearer token authorization, rolling IP rate limiting (10 req/min), single-active-experiment target concurrency locks (`409 Conflict`), and CORS handling. |
| [`internal/config`](file:///internal/config) | **Configuration Management**: Parses `config.yaml` and handles environment variable overrides (`HAVOC_ALLOWED_NAMESPACES`, `HAVOC_API_TOKEN`, `HAVOC_BIND_ADDRESS`, `DATABASE_URL`, `PROMETHEUS_URL`, `ANTHROPIC_API_KEY`). Expands home directory paths for kubeconfig. |
| [`internal/events`](file:///internal/events) | **Event System & WebSocket Server**: Thread-safe in-memory publish/subscribe event hub (`Hub`) and Gorilla WebSocket server (`:9091`). Broadcasts structured real-time experiment lifecycle events to connected dashboard clients. |
| [`internal/experimentdef`](file:///internal/experimentdef) | **Chaos-as-Code Definition**: Data models and YAML parser for declarative experiment specifications. Validates actions (`kill-pod`, `inject-latency`, `spike-cpu`), target selectors, parameters, and per-experiment safety limits. |
| [`internal/experiments`](file:///internal/experiments) | **Low-Level Chaos Primitives**: Interacts directly with the Kubernetes API to delete targeted pods, inject network latency using `tc`/`netem` via ephemeral containers, and apply CPU stress via `stress-ng` ephemeral containers. |
| [`internal/k8sclient`](file:///internal/k8sclient) | **Kubernetes Client Factory**: Initializes `k8s.io/client-go` interfaces from local kubeconfig or in-cluster service accounts. |
| [`internal/metrics`](file:///internal/metrics) | **Prometheus Metrics & Observability**: Defines Prometheus counters (`havoc_experiments_total`), histograms (`havoc_pod_recovery_seconds`), and gauges (`havoc_experiment_error_rate`). Serves `:9090/metrics` and provides `PromClient` to query live cluster error rates. |
| [`internal/postmortem`](file:///internal/postmortem) | **AI Incident Postmortem Generator**: Integrates with the Anthropic Claude API to transform raw incident metrics into structured, plain-English Markdown postmortems using a Go text template. |
| [`internal/safety`](file:///internal/safety) | **Safety Runner & Guardrails**: Wraps all chaos execution. Enforces namespace allowlists, hard-coded system denylists, blast radius boundaries, pre- and post-run error rate auto-aborts, ephemeral container cleanup, and synchronous recovery watching. |
| [`internal/scheduler`](file:///internal/scheduler) | **Automated Chaos Cron**: Background scheduler running experiments within configured business-hour time windows. Backed by PostgreSQL for persistent pause/resume state and audit logging. |
| [`internal/scoring`](file:///internal/scoring) | **Resilience Scoring Engine**: Computes a deterministic score ($0-100$) by evaluating pod recovery latency against optimal/maximum thresholds, observed error rates, and safety intervention penalties. |
| [`internal/storage`](file:///internal/storage) | **PostgreSQL Persistence Layer**: Handles database connection pooling, schema migrations, and CRUD operations for experiment history, score trends, AI postmortems, and scheduler state. |

---

## 🎯 Main Design Decisions

### 1. Safety Checks Execute Before Cluster Mutation
Chaos engineering requires strict blast radius boundaries. Before any Kubernetes API call is made:
- Target namespaces are checked against `safety.allowedNamespaces`. System namespaces (`kube-system`, `kube-public`, `kube-node-lease`) are rejected by a hard-coded denylist even if listed. Disallowed requests are blocked before touching cluster resources, emitting a `blocked` event and recording metrics.
- Target pod counts are compared against total matching workload pods to enforce `maxBlastRadiusPercent`.
- If the current error rate from Prometheus already exceeds `abortOnErrorRatePercent`, execution aborts immediately.

### 2. Asynchronous API with Synchronous Recovery Watching
- The API endpoint `POST /api/experiments` returns `202 Accepted` immediately with a unique experiment UUID.
- The experiment runs in a background goroutine where `watchPodRecovery` synchronously monitors the Kubernetes Watch stream for the exact moment the replacement pod or ephemeral container achieves `Ready` status.
- State transitions (`experiment_started`, `target_selected`, `pod_killed`, `recovering`, `recovered`, `completed`) are broadcast over WebSocket, decoupling the HTTP client from long-running recovery operations.

### 3. Server-Side Proxying for Credential Protection
- The Next.js frontend never contacts the Go Engine API or PostgreSQL database directly.
- All requests flow through Next.js server-side Route Handlers (`/api/...`), which securely inject `HAVOC_API_TOKEN` and connect over internal Docker networks. Credentials and internal network topology are never leaked to client browsers.

### 4. Read-Only Public Showcase Mode
- For internet-facing demos, setting `DEMO_READ_ONLY=true` disarms all chaos injection buttons on the frontend with a tooltip (*"Run locally to inject real chaos"*).
- The route handler rejects incoming `POST /api/experiments` with `403 Forbidden`, allowing the public to safely explore the live topology, system health, score history, and AI postmortems.

### 5. Graceful Fallback & Offline Resilience
- If PostgreSQL or Kubernetes is unreachable during CLI runs, commands log a warning and execute in-memory or dry-run without crashing.
- The frontend dashboard includes fallback topologies and status cards when the engine is offline, alerting the user while remaining responsive.
