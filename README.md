# Havoc Engine 💥

`havoc-engine` is a lightweight, testable Kubernetes chaos engineering CLI tool and dashboard. It allows developers and site reliability engineers to inject controlled failure experiments into Kubernetes clusters to validate resiliency, providing a score based on how well the system recovers.

**Problem it solves:** Modern microservices are complex and distributed. It's hard to know how they will behave when a pod dies, the network lags, or CPU usage spikes. `havoc-engine` helps you run controlled "chaos experiments" to uncover weaknesses *before* they cause a production outage, and tracks your system's resilience over time.

---

## 🏗 Architecture Overview

The project is composed of several key components:

1. **Havoc Engine (Go CLI / Server)**: The core brain that connects to the Kubernetes API. It executes chaos experiments (like killing pods or injecting network latency via ephemeral containers), monitors recovery times, and calculates resilience scores. It also has a built-in scheduler (`havoc-cron`) for automated chaos and a `/metrics` server for observability.
2. **Havoc Dashboard (Next.js / React)**: A web-based user interface to visualize chaos experiments, monitor blast radius limits, and view the system's resilience score trends.
3. **PostgreSQL Database**: Stores experiment history, resilience scores, and cron scheduler state.
4. **Prometheus**: Scrapes metrics exposed by the Havoc Engine, such as execution counts, recovery times, and error rates.

---

## 💻 Tech Stack

- **Backend / CLI**: Go (1.21+), `k8s.io/client-go` for Kubernetes interaction, Cobra for CLI framing.
- **Frontend Dashboard**: Next.js, React, Tailwind CSS.
- **Database**: PostgreSQL.
- **Observability**: Prometheus (`prometheus/client_golang`).
- **AI Integration**: Anthropic Claude API for generating plain-English AI postmortem reports of chaos experiments.

---

## 📊 Metrics & Observability

The Havoc Engine exposes a `/metrics` endpoint scraped by Prometheus. Notably, the **auto-abort safety mechanism** relies on a real error rate queried dynamically from Prometheus. However, note that this error rate measures **recent experiment failure rate** (i.e. the percentage of recent havoc experiments that failed or were aborted), rather than full application-level live traffic monitoring. It is an approximation based on the engine's own experiment history.

---

## ⚠️ Known Limitations (Not Production Ready)

**Please note that this project is currently in a pre-release state and is NOT yet ready for production use.**

- **Mock Dashboard Data**: The live data shown in the dashboard (status cards, experiment triggers, blast radius graph) currently uses mock/placeholder data and is not yet wired to the real backend engine API.
- **No Live Deployment**: There is no live, public deployment of the dashboard or engine yet. It is designed to be run locally for development and testing.

---

## 🚀 Setup & Local Development

You can bring up the entire stack locally using Docker Compose.

### Prerequisites
- Docker and Docker Compose
- Go 1.21+
- Node.js 18+ (for dashboard development)
- A local Kubernetes cluster (e.g., Minikube, k3d, Docker Desktop)
- `kubectl` configured

### Environment Variables
Before running, you may need to configure the following environment variables (or place them in a `.env` file):
- `ANTHROPIC_API_KEY`: Required if you want to generate AI postmortem reports.
- `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`: (Optional) Overrides for the database if not using the defaults in docker-compose.
- `DATABASE_URL`: Connection string for PostgreSQL (used by the engine).

### Running with Docker Compose

A single command will bring up the Engine, Dashboard, PostgreSQL, Prometheus, and the Cron service:

```bash
docker-compose up --build -d
```

- **Dashboard**: Available at [http://localhost:3000](http://localhost:3000)
- **Prometheus**: Available at [http://localhost:9091](http://localhost:9091)
- **Engine Metrics**: Available at [http://localhost:9090/metrics](http://localhost:9090/metrics)

---

## 🛠️ CLI Command Reference

The `havoc-engine` binary provides several commands to trigger and manage chaos experiments:

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
- `havoc-engine cron start`: Starts the automated chaos scheduler in the foreground.
- `havoc-engine cron pause`: Pauses the automated scheduler (state is persisted in the DB).
- `havoc-engine cron resume`: Resumes the paused automated scheduler.
- `havoc-engine cron status`: Shows the current status of the scheduler and an audit log of recent automated runs.

### Server
- `havoc-engine serve`: Starts the engine in server mode, keeping the `/metrics` endpoint alive for Prometheus to scrape.

*(Note: All commands support a `--dry-run` flag to simulate execution without making actual changes to the cluster.)*
