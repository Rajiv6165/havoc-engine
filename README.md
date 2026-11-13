# Havoc Engine 💥
[![CI](https://github.com/Rajiv6165/havoc-engine/actions/workflows/ci.yml/badge.svg)](https://github.com/Rajiv6165/havoc-engine/actions/workflows/ci.yml)

`havoc-engine` is a lightweight, testable Kubernetes chaos engineering CLI tool and real-time dashboard. It allows developers and site reliability engineers to inject controlled failure experiments into Kubernetes clusters to validate resiliency, providing a score based on how well the system recovers.

**Problem it solves:** Modern microservices are complex and distributed. It's hard to know how they will behave when a pod dies, the network lags, or CPU usage spikes. `havoc-engine` helps you run controlled "chaos experiments" to uncover weaknesses *before* they cause a production outage, and tracks your system's resilience over time.

---

## 🏗 Architecture Overview

```mermaid
flowchart TD
    Dashboard["Havoc Dashboard (Next.js 14 / React 18)"]
    Cron["Havoc Cron Scheduler"]
    Engine["Havoc Engine Control Plane (Go 1.22)"]
    K8s["Kubernetes Cluster (Pods / Deployments)"]
    Postgres[("PostgreSQL Database")]
    Prometheus["Prometheus Metrics Server"]
    Claude["Anthropic Claude API"]

    Dashboard -->|"HTTP REST (:8080) & WebSocket (:9091)"| Engine
    Cron -->|"Scheduled Chaos Runs"| Engine
    Engine -->|"Pod Kill & Ephemeral Containers"| K8s
    Engine -->|"Persist Experiments & Postmortems"| Postgres
    Engine -->|"Error Rate Queries & Scrape Endpoint (:9090)"| Prometheus
    Engine -->|"Generate Incident Postmortems"| Claude
```

The system is composed of several core components:

1. **Havoc Engine (Go CLI / Server)**: The core brain that connects to the Kubernetes API. It executes chaos experiments (killing pods, injecting network latency via `tc`/`netem` ephemeral containers, or spiking CPU via `stress-ng`), monitors recovery times, and calculates resilience scores. It provides an HTTP REST API (:8080), a WebSocket event streaming server (:9091), and a Prometheus metrics endpoint (:9090), plus a built-in scheduler (`havoc-cron`).
2. **Havoc Dashboard (Next.js / React)**: A web-based control plane to visualize cluster health, trigger experiments, inspect blast radius graphs with real-time WebSocket animations, and review resilience scores and AI postmortem reports. Supports a read-only demo mode (`DEMO_READ_ONLY=true`).
3. **PostgreSQL Database**: Stores experiment history, resilience scores, AI postmortem reports, and cron scheduler state.
4. **Prometheus**: Scrapes metrics exposed by the Havoc Engine, such as execution counts, recovery times, and error rates.
5. **Claude API**: Generates automated, plain-English incident postmortem reports summarizing root causes, SLA impacts, and remediation advice.

---

## 🔄 How an Experiment Flows

Every chaos experiment follows an end-to-end lifecycle designed for safety, precision, and observability:

```
[Request] ➡️ [Safety Checks] ➡️ [Run] ➡️ [Recovery Watch] ➡️ [Score] ➡️ [Storage] ➡️ [Postmortem] ➡️ [Live Events]
```

1. **Request**: Initiated via the CLI (`havoc-engine kill-pod`), Chaos-as-Code YAML (`havoc-engine run -f ...`), the built-in Cron scheduler, or the Web Dashboard (`POST /api/experiments`).
2. **Safety Checks**:
   - **Namespace Allowlist & Denylist**: Validates that the target namespace is present in `safety.allowedNamespaces` (or `HAVOC_ALLOWED_NAMESPACES`). System namespaces (`kube-system`, `kube-public`, `kube-node-lease`) are rejected by a hard-coded denylist even if listed. Violations are blocked *before touching the cluster*, emit a `blocked` event, and increment `result="blocked"` metrics.
   - **One-Per-Target Active Guard**: Rejects concurrent experiments targeting the same service or pod with `409 Conflict`.
   - **Blast Radius Validation**: Checks that targeting $N$ pods does not exceed `maxBlastRadiusPercent` of the matching group.
   - **Pre-Run Error Threshold**: Queries Prometheus; if the target namespace error rate already exceeds `abortOnErrorRatePercent`, the run is aborted.
3. **Run**: The experiment is executed against the cluster:
   - `kill-pod`: Randomly deletes a matching pod via Kubernetes API.
   - `inject-latency`: Attaches an ephemeral debug container with `tc netem` network delay.
   - `spike-cpu`: Attaches an ephemeral container running `stress-ng`.
4. **Recovery Watch**: The engine synchronously watches the Kubernetes Watch stream until the pod or container recovers to a `Ready` condition, measuring exact recovery duration in milliseconds.
5. **Score**: Calculates a resilience score ($0-100$) based on recovery time, observed error rate, and whether safety interventions were triggered.
6. **Storage**: Persists the experiment record, recovery duration, error metrics, and score in PostgreSQL.
7. **Postmortem**: If `ANTHROPIC_API_KEY` is provided, Claude analyzes the incident data and drafts an automated postmortem report.
8. **Live Events**: Real-time lifecycle events (`experiment_started`, `target_selected`, `pod_killed`, `recovering`, `recovered`, `completed`, `blocked`, `aborted`) are broadcast over WebSocket (:9091) to animate the dashboard topology and event log.

---

## 📸 Screenshots & Demo

<!-- Placeholder for dashboard overview screenshot:
     docs/img/dashboard.png should display the full Havoc Dashboard including:
     - System Status cards with replica counts and health badges
     - Interactive Blast Radius topology map with dependency edges
     - Chaos Controls panel with Kill Pod, Latency, and CPU Stress action buttons
     - Resilience Score Trend chart and Experiment History table
-->
![Havoc Dashboard](docs/img/dashboard.png)

<!-- Placeholder for animated walkthrough GIF:
     docs/img/demo.gif should showcase a live chaos injection workflow:
     1. Clicking 'Kill Pod' on the frontend service
     2. Real-time WebSocket event logs streaming in
     3. Topology node flashing during pod outage
     4. Recovery detected within seconds, updating the resilience score and generating an AI postmortem
-->
![Havoc Live Demo](docs/img/demo.gif)

---

## 🎮 Quickstart / Run the Demo

Get a fully functioning local demo environment running in under 2 minutes using the automated setup script.

### Using the Automated Demo Script

**On Linux / macOS:**
```bash
chmod +x scripts/demo.sh
./scripts/demo.sh
```

**On Windows (PowerShell):**
```powershell
.\scripts\demo.ps1
```

The script automatically:
1. Verifies prerequisites (`docker`, `kubectl`, `minikube` or `k3d`).
2. Starts or connects to a local Kubernetes cluster.
3. Creates the `demo` namespace and deploys sample microservices (`frontend`, `api`, `db`, `queue`).
4. Launches the full stack with `docker compose up --build -d`.
5. Waits for the API to become ready.
6. Runs `havoc-engine seed-demo` so your history panel, score trend, and postmortems are ready on first open. Seeded baseline records are marked with a `sample data` badge in the dashboard's Experiment History table to clearly differentiate them from live chaos runs.
7. Prints your dashboard URL ([http://localhost:3000](http://localhost:3000)) and suggested clicks!

---

## 💻 Tech Stack

- **Backend / CLI**: Go (1.22+), `k8s.io/client-go`, Gorilla WebSocket, Cobra CLI.
- **Frontend Dashboard**: Next.js 14, React 18, ReactFlow (`@xyflow/react`), Tailwind CSS.
- **Database**: PostgreSQL 15.
- **Observability**: Prometheus (`prometheus/client_golang`).
- **AI Integration**: Anthropic Claude API for automated incident postmortem generation.

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

## ⚠️ Known Limitations

- **Kubernetes Ephemeral Containers**: Network latency injection (`inject-latency`) and CPU stress (`spike-cpu`) require ephemeral container support in the target cluster (enabled by default in Kubernetes 1.25+). Pod termination (`kill-pod`) works on all standard Kubernetes clusters.
- **Single-Cluster Scope**: The engine connects to a single Kubernetes cluster per instance via standard `kubeconfig` or in-cluster service account.
- **Simple Bearer Token Authentication**: API authentication uses a shared bearer token (`HAVOC_API_TOKEN`) rather than multi-tenant OAuth2/OIDC or per-user RBAC.
- **Error Rate Metrics**: Auto-abort thresholds query aggregate error rates from Prometheus (`prometheus.url`) rather than in-band distributed tracing spans.

---

## 🔒 Security Model

Havoc Engine incorporates defensive safeguards to protect cluster stability and prevent unauthorized chaos:

- **Namespace Allowlist & System Denylist**: The `safety.allowedNamespaces` configuration (and `HAVOC_ALLOWED_NAMESPACES` env override) restricts chaos injection to approved namespaces only. System namespaces (`kube-system`, `kube-public`, `kube-node-lease`) are permanently forbidden.
- **Local Bind by Default**: The HTTP API server binds to `127.0.0.1` by default (`server.host` in `config.yaml` or `HAVOC_BIND_ADDRESS`), preventing unintended public interface exposure.
- **Bearer Token Authorization**: When `HAVOC_API_TOKEN` is configured, all mutable endpoints require `Authorization: Bearer <token>`. Read endpoints can also require auth via `api.requireAuthForReads: true`.
- **Server-Side Token Proxying**: The Next.js dashboard routes API requests through server-side Route Handlers (`/api/...`), injecting `HAVOC_API_TOKEN` so credentials are never exposed in browser clients.
- **Read-Only Demo Mode**: Setting `DEMO_READ_ONLY=true` on the dashboard disables all chaos execution buttons with explanatory tooltips, keeping public demos safe.
- **Rate Limiting & Concurrency Control**: The API limits requests to 10 per minute per client IP and forbids overlapping experiments on the same target (`409 Conflict`).

---

## ⚙️ Environment Variables

| Variable | Description | Default |
|---|---|---|
| `HAVOC_ALLOWED_NAMESPACES` | Comma-separated list of namespaces permitted for chaos | `demo` (in demo config) |
| `HAVOC_BIND_ADDRESS` | Network interface address to bind HTTP API server to | `127.0.0.1` |
| `HAVOC_API_TOKEN` | Shared secret bearer token required for `POST /api/experiments` | *(empty / disabled)* |
| `DEMO_READ_ONLY` | Sets dashboard to read-only demo mode (disables action buttons) | `false` |
| `PROMETHEUS_URL` | Base URL of Prometheus server to query metrics from | `http://localhost:9092` / `http://prometheus:9090` |
| `DATABASE_URL` | PostgreSQL connection string / DSN | `postgres://postgres:postgres@localhost:5432/havoc?sslmode=disable` |
| `ANTHROPIC_API_KEY` | Anthropic Claude API key for automated postmortem generation | *(empty / disabled)* |
| `HAVOC_API_URL` | Engine URL used by Next.js dashboard server-side proxy | `http://localhost:8080` / `http://havoc-engine:8080` |

---

## 🛠️ CLI Command Reference

### Server & Control Plane
- `havoc-engine serve`: Starts the Havoc Engine control plane (HTTP API on :8080, WebSocket stream on :9091, metrics on :9090).
- `havoc-engine seed-demo`: Populates the database with realistic baseline experiments, scores, and AI postmortems (persisted with a `seeded` flag and displayed with a `sample data` badge in the dashboard history), and tests live cluster connectivity.

### Chaos Actions
- `havoc-engine kill-pod --namespace <ns> --selector <label>`: Deletes a random pod matching the label selector.
- `havoc-engine inject-latency --namespace <ns> --pod <name> --delay <ms>`: Injects network delay using `tc`/`netem` via an ephemeral container.
- `havoc-engine spike-cpu --namespace <ns> --pod <name> --duration <s>`: Stresses CPU inside a target pod using a `stress-ng` ephemeral container.

### Experiment Management
- `havoc-engine run --file <path/to/experiment.yaml>`: Run a complex experiment from a YAML definition file.
- `havoc-engine history`: View recent chaos experiment history and their resilience scores.
- `havoc-engine score`: Calculate and print the current average resilience score and trend.
- `havoc-engine report --latest` (or `--id <uuid>`): View or generate an AI postmortem report for an experiment.

### Scheduler (Cron)
- `havoc-engine cron start`: Starts the automated chaos scheduler in the foreground.
- `havoc-engine cron pause`: Pauses the automated scheduler (state is persisted in DB).
- `havoc-engine cron resume`: Resumes the paused automated scheduler.
- `havoc-engine cron status`: Shows scheduler status and audit log of recent automated runs.

*(Note: All chaos commands support a `--dry-run` flag to simulate execution without mutating cluster state.)*
