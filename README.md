# Havoc Engine 💥
[![CI](https://github.com/Rajiv6165/havoc-engine/actions/workflows/ci.yml/badge.svg)](https://github.com/Rajiv6165/havoc-engine/actions/workflows/ci.yml)

`havoc-engine` is a lightweight, testable Kubernetes chaos engineering CLI tool and real-time dashboard. It allows developers and site reliability engineers to inject controlled failure experiments into Kubernetes clusters to validate resiliency, providing a score based on how well the system recovers.

**Problem it solves:** Modern microservices are complex and distributed. It's hard to know how they will behave when a pod dies, the network lags, or CPU usage spikes. `havoc-engine` helps you run controlled "chaos experiments" to uncover weaknesses *before* they cause a production outage, and tracks your system's resilience over time.

---

## 🏗 Architecture Overview

The project is composed of several key components:

1. **Havoc Engine (Go CLI / Server)**: The core brain that connects to the Kubernetes API. It executes chaos experiments (like killing pods or injecting network latency via ephemeral containers), monitors recovery times, and calculates resilience scores. It provides an HTTP REST API (:8080), a WebSocket event streaming server (:9091), and a Prometheus metrics endpoint (:9090), plus a built-in scheduler (`havoc-cron`).
2. **Havoc Dashboard (Next.js / React)**: A web-based control plane to visualize cluster health, trigger experiments, inspect blast radius graphs with real-time WebSocket animations, and review resilience scores and AI postmortem reports.
3. **PostgreSQL Database**: Stores experiment history, resilience scores, postmortem reports, and cron scheduler state.
4. **Prometheus**: Scrapes metrics exposed by the Havoc Engine, such as execution counts, recovery times, and error rates.

---

## 💻 Tech Stack

- **Backend / CLI**: Go (1.22+), `k8s.io/client-go` for Kubernetes interaction, Gorilla WebSocket, Cobra for CLI framing.
- **Frontend Dashboard**: Next.js 14, React 18, ReactFlow (`@xyflow/react`), Tailwind CSS.
- **Database**: PostgreSQL 15.
- **Observability**: Prometheus (`prometheus/client_golang`).
- **AI Integration**: Anthropic Claude API for generating plain-English AI postmortem reports of chaos experiments.

---

## 🎮 Run the Demo

To test `havoc-engine` and `havoc-dashboard` against a live local Kubernetes cluster:

### 1. Start a local Kubernetes cluster
```bash
# Using Minikube:
minikube start

# OR using k3s / k3d:
k3d cluster create havoc-demo
```

### 2. Deploy the sample microservices app
Apply the demo application (frontend, api, db, queue with `havoc.io/depends-on` dependency annotations):
```bash
kubectl apply -f deploy/demo/app.yaml
```
Verify the deployments and pods are running:
```bash
kubectl get deployments,pods
```

### 3. Start the Havoc stack
Start PostgreSQL, Prometheus, Havoc Engine (`serve`), and Havoc Dashboard:
```bash
docker-compose up --build -d
```
*(Or run locally: start the engine with `go run ./cmd/havoc-engine serve` and dashboard with `cd havoc-dashboard && npm run dev`)*

### 4. Open the Dashboard & Attack
1. Open [http://localhost:3000](http://localhost:3000) in your browser.
2. Verify the **System Status** cards show live pods for `frontend`, `api`, `db`, and `queue`, and the **Blast Radius** graph displays the live annotated topology.
3. Click **Kill Pod** under Chaos Controls.
4. Watch the **Live Experiment Logs** stream events in real time, the **Blast Radius** graph highlight the targeted node, and the **Experiment History** record the recovery time and resilience score!

---

## 📊 Endpoints & Observability

- **Dashboard**: [http://localhost:3000](http://localhost:3000)
- **Engine HTTP API**: [http://localhost:8080](http://localhost:8080)
  - `GET /api/status`: Live deployment readiness and health in configured namespace(s)
  - `POST /api/experiments`: Async experiment execution through safety runner, scoring, and storage
  - `GET /api/experiments`: Recent experiment history with resilience scores and AI postmortems
  - `GET /api/topology`: Dynamic service dependency graph built from `havoc.io/depends-on` annotations
- **WebSocket Event Stream**: `ws://localhost:9091`
- **Engine Metrics**: [http://localhost:9090/metrics](http://localhost:9090/metrics)
- **Prometheus UI**: [http://localhost:9092](http://localhost:9092)

---

## ⚠️ Known Limitations & Current Status

- **Live Data**: System status, chaos execution, topology graph, live log streaming, and history scores run on real backend data with automatic graceful fallback when the engine is offline.
- **API Authentication**: The API supports simple shared bearer token authentication (`HAVOC_API_TOKEN`) rather than full multi-user role-based access control (RBAC). Any client in possession of the shared token can trigger chaos experiments.
- **Memory Leak Action**: The "Memory Leak" button in the dashboard is not yet implemented in the Go engine (current supported actions: `kill-pod`, `inject-latency`, `spike-cpu`).
- **Safety Error Rate Metric**: The auto-abort mechanism queries the recent chaos experiment failure rate from Prometheus rather than full application-level traffic tracing.

---

## 🔒 Security

Havoc Engine incorporates several safeguards to ensure secure operation and protect cluster stability:

- **Local Bind by Default**: The HTTP API server binds to `127.0.0.1` by default (`server.host` in `config.yaml` or `HAVOC_BIND_ADDRESS`), preventing unintended exposure on public network interfaces. Inside containerized environments such as Docker Compose, configure `HAVOC_BIND_ADDRESS: "0.0.0.0"`.
- **Bearer Token Authorization**: When `HAVOC_API_TOKEN` is configured, all mutable endpoints (`POST /api/experiments`) require an `Authorization: Bearer <token>` header, returning `401 Unauthorized` if missing or invalid. Read-only endpoints (`GET /api/status`, `GET /api/experiments`, `GET /api/topology`) remain open for dashboard monitoring, or can require authentication by setting `api.requireAuthForReads: true` in `config.yaml`.
- **Server-Side Token Proxying**: The Next.js dashboard routes API requests through server-side Route Handlers (`/api/...`), injecting `HAVOC_API_TOKEN` from the server environment so that secrets are never sent to or visible in client browsers.
- **Client Rate Limiting**: The `POST /api/experiments` endpoint limits requests to 10 per minute per client IP to prevent flooding, returning `429 Too Many Requests`.
- **Single Active Experiment Per Target**: Only one chaos experiment can run against a given service or pod target at any time. Concurrent requests for an active target are rejected with `409 Conflict`.

---

## ⚙️ Environment Variables

| Variable | Description | Default |
|---|---|---|
| `HAVOC_BIND_ADDRESS` | Network interface address to bind HTTP API server to | `127.0.0.1` |
| `HAVOC_API_TOKEN` | Shared secret bearer token required for `POST /api/experiments` | *(empty / disabled)* |
| `PROMETHEUS_URL` | Base URL of Prometheus server to query metrics from | `http://localhost:9092` (local) / `http://prometheus:9090` (docker) |
| `DATABASE_URL` | PostgreSQL connection string / DSN | `postgres://postgres:postgres@localhost:5432/havoc?sslmode=disable` |
| `ANTHROPIC_API_KEY` | Anthropic Claude API key for automated postmortem generation | *(empty / disabled)* |
| `HAVOC_API_URL` | Engine URL used by Next.js dashboard server-side proxy | `http://localhost:8080` (or `http://havoc-engine:8080` in docker) |

---

## 🛠️ CLI Command Reference

The `havoc-engine` binary provides commands to trigger and manage chaos experiments:

### Server & Control Plane
- `havoc-engine serve`: Starts the complete Havoc Engine control plane (HTTP API on :8080, WebSocket stream on :9091, metrics on :9090) with graceful shutdown on SIGINT/SIGTERM.

### Chaos Actions
- `havoc-engine kill-pod --namespace <ns> --selector <label>`: Deletes a random pod matching the label selector.
- `havoc-engine inject-latency --namespace <ns> --pod <name> --delay <ms>`: Injects network delay into a target pod using `tc`/`netem` via an ephemeral container.
- `havoc-engine spike-cpu --namespace <ns> --pod <name> --duration <s>`: Stresses CPU inside a target pod using a `stress-ng` ephemeral container for the specified duration.

### Experiment Management
- `havoc-engine run --file <path/to/experiment.yaml>`: Run a complex experiment from a YAML definition file.
- `havoc-engine history`: View recent chaos experiment history and their resilience scores.
- `havoc-engine score`: Calculate and print the current average resilience score and trend based on recent runs.
- `havoc-engine report --latest` (or `--id <uuid>`): Generate an AI-powered postmortem report for an experiment using the Anthropic API.

### Scheduler (Cron)
- `havoc-engine cron start`: Starts the automated chaos scheduler in the foreground (starts metrics server).
- `havoc-engine cron pause`: Pauses the automated scheduler (state is persisted in the DB).
- `havoc-engine cron resume`: Resumes the paused automated scheduler.
- `havoc-engine cron status`: Shows the current status of the scheduler and an audit log of recent automated runs.

*(Note: All chaos commands support a `--dry-run` flag to simulate execution without making actual changes to the cluster.)*
