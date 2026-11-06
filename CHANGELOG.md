# Changelog

All notable changes to `havoc-engine` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0] - 2026-11-07

### Summary
Initial public v1.0.0 release of Havoc Engine — a Kubernetes chaos engineering control plane, real-time dashboard, automated resilience scoring system, and AI-powered postmortem engine.

### Added

#### 🚀 Engine & Control Plane
- **Core CLI**: Multi-subcommand CLI built on Cobra (`serve`, `kill-pod`, `inject-latency`, `spike-cpu`, `run`, `history`, `score`, `report`, `cron`, `seed-demo`).
- **Control Plane Server**: Unified `serve` command providing:
  - HTTP REST API on `:8080` with dynamic service topology, status, and experiment endpoints.
  - Real-time WebSocket event streaming server on `:9091` broadcasting structured lifecycle events.
  - Prometheus metrics scraper endpoint on `:9090/metrics`.
- **Chaos-as-Code**: YAML-defined experiment runner (`havoc-engine run -f <file>`) with declarative targets, action parameters, and per-experiment safety threshold overrides.
- **Microservices Demo Seeder**: Added `havoc-engine seed-demo` command to initialize PostgreSQL schemas, seed baseline experiment history with AI postmortems, and verify cluster connectivity.

#### 🛡️ Safety & Guardrails
- **Namespace Allowlist**: Configurable `safety.allowedNamespaces` in `config.yaml` with `HAVOC_ALLOWED_NAMESPACES` environment override. Non-empty allowlists block any experiment targeting outside namespaces before touching the cluster, emitting a `blocked` event and recording `result="blocked"` metrics.
- **Hard-Coded System Namespace Denylist**: Enforced absolute protection for `kube-system`, `kube-public`, and `kube-node-lease`, permanently preventing chaos execution against system namespaces even if explicitly listed in the allowlist.
- **Blast Radius Protection**: Pod impact percentage validation (`CheckBlastRadiusBySelector` and `CheckBlastRadiusByPodName`) preventing actions that exceed `maxBlastRadiusPercent`.
- **Pre- & Post-Execution Auto-Abort**: Prometheus error rate monitoring (`abortOnErrorRatePercent`) that blocks execution or automatically rolls back in-progress latency or CPU stress if failure rates spike.
- **Ephemeral Container Cleanup**: Automatic removal and rollback of injected `tc`/`netem` and `stress-ng` containers upon auto-abort or completion.

#### 📈 Scoring & Metrics
- **Automated Resilience Scoring**: Formula-driven score ($0-100$) evaluating:
  - Pod and service recovery latency against optimal and maximum SLAs.
  - Transient error rate percentage during and after chaos injection.
  - Heavy deduction penalty applied when automated safety aborts intervene.
- **Observability Exporters**: Prometheus metrics exporter tracking total experiments (`havoc_experiments_total{experiment_type, result}`), pod recovery latency histograms (`havoc_pod_recovery_seconds`), and simulated/observed error rates (`havoc_experiment_error_rate`).

#### 🤖 AI Postmortems
- **Claude API Integration**: Automated incident postmortem generation using Anthropic Claude models.
- **Structured Incident Templates**: Formats root cause analysis, timeline breakdown, SLA compliance, and prioritized engineering recommendations into plain-English Markdown.
- **CLI & Web Viewing**: View past postmortems directly via `havoc-engine report --latest` or within the interactive dashboard experiment drawer.

#### ⏰ Cron & Automated Chaos
- **Chaos Scheduler**: Built-in background daemon (`havoc-cron` / `havoc-engine cron start`) executing randomized experiments inside defined business-hours time windows.
- **State Controls**: Persistent pause (`cron pause`) and resume (`cron resume`) controls backed by PostgreSQL with full execution audit logging (`cron status`).

#### 💻 Dashboard & User Experience
- **Interactive Control Plane**: Next.js 14 and React 18 dashboard styled with Tailwind CSS.
- **Dynamic Blast Radius Map**: Interactive SVG/ReactFlow dependency graph visualizing Kubernetes deployments, health statuses, and real-time chaos propagation using `havoc.io/depends-on` metadata.
- **Live Event Stream**: Real-time WebSocket event feed displaying experiment status transitions (`experiment_started`, `target_selected`, `pod_killed`, `recovering`, `recovered`, `completed`, `blocked`, `aborted`).
- **Read-Only Demo Mode**: Support for `DEMO_READ_ONLY=true` (and `NEXT_PUBLIC_DEMO_READ_ONLY=true`), disabling chaos injection buttons with informative tooltips (*"Run locally to inject real chaos"*) for safe public showcasing.

#### 🔒 Security
- **Local Bind Default**: Engine HTTP API binds to `127.0.0.1` by default to prevent unintended public interface exposure.
- **Bearer Token Authentication**: Configurable `HAVOC_API_TOKEN` protecting mutable execution endpoints with optional read-endpoint protection (`api.requireAuthForReads`).
- **Server-Side Token Proxying**: Next.js API route handlers proxy requests internally, preventing credential leakage to web clients.
- **IP Rate Limiting**: Rolling rate limiter restricting clients to 10 requests per minute per IP address (`429 Too Many Requests`).
- **One Active Experiment Per Target**: Mutual exclusion guard preventing concurrent overlapping chaos on the same service or pod target (`409 Conflict`).

#### 🚢 Deployment & Infrastructure
- **Automated Demo Scripts**: One-step bootstrap scripts (`scripts/demo.sh` and `scripts/demo.ps1`) verifying Docker, kubectl, minikube/k3d, deploying the demo microservices application, and standing up the full stack.
- **Production Compose & Caddy**: Hardened `docker-compose.prod.yml` and `Caddyfile` configuration keeping backend API, PostgreSQL, and Prometheus unexposed to the public internet while securing dashboard traffic with automated HTTPS.
- **AWS Deployment Guide**: Comprehensive step-by-step deployment guide in `deploy/aws/README.md` for running on a single small EC2 instance with k3s and security checklists.

#### 🧪 CI / CD
- **GitHub Actions Workflow**: Automated multi-stage CI running `go test ./...`, `go vet ./...`, `npm run lint`, and `npm run build` on every push and pull request.
