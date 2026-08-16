# Havoc Engine 💥

`havoc-engine` is a lightweight, testable Kubernetes chaos engineering CLI tool built in Go using `client-go`. It allows developers and site reliability engineers to inject controlled failure experiments into Kubernetes clusters to validate resiliency.

---

## Features & Chaos Experiments

1. **`killPod`**: Deletes a random pod matching a target label selector in a given namespace.
2. **`injectLatency`**: Injects network delay (using `tc`/`netem` via an ephemeral container with `NET_ADMIN` capability) into a target pod.
3. **`spikeCPU`**: Stresses CPU cores inside a target pod using a `stress-ng` ephemeral container for a specified duration.

---

## Safety & Guardrails 🛡️

`havoc-engine` includes built-in safety guardrails implemented as a middleware layer in `internal/safety/` to prevent unexpected outages during chaos experiments:

1. **Blast Radius Limit**:
   - Ensures chaos experiments never impact more than a configurable percentage of workload pods matching the label selector in a namespace (default **30%**).
   - If an experiment would affect more than the configured percentage (e.g., targeting 1 pod in a 2-replica deployment = 50%), execution is automatically blocked before any cluster changes occur.

2. **Auto-Abort & Rollback**:
   - Accepts a `MetricsChecker` interface (pluggable for Prometheus or custom metrics sources) that monitors system error rates.
   - If the error rate exceeds the configurable threshold (`abortOnErrorRatePercent`, default **5.0%**) before or mid-experiment, `havoc-engine` immediately aborts execution and attempts an automatic rollback (e.g. removing injected latency or CPU stress ephemeral containers).

3. **Dry-Run Mode (`--dry-run`)**:
   - Every CLI command supports a `--dry-run` flag.
   - When enabled, `havoc-engine` logs exactly what action *would* take place (which pod would be targeted, how much delay or CPU stress would be injected) without making any modifying Kubernetes API calls.

---

## Project Structure

```text
havoc-engine/
├── cmd/
│   └── havoc-engine/
│       └── main.go          # CLI entrypoint, flags, and Cobra command wiring
├── internal/
│   ├── config/              # YAML configuration loader (config.yaml)
│   ├── k8sclient/           # Out-of-cluster / In-cluster Kubernetes client initializer
│   ├── metrics/             # Prometheus metrics definitions and HTTP server
│   ├── safety/              # Safety middleware (blast radius limit, auto-abort, dry-run mode)
│   │   ├── safety.go
│   │   └── safety_test.go
│   └── experiments/         # Chaos experiment implementations, rollback helpers, and tests
│       ├── kill_pod.go
│       ├── kill_pod_test.go
│       ├── inject_latency.go
│       ├── inject_latency_test.go
│       ├── spike_cpu.go
│       ├── spike_cpu_test.go
│       └── rollback.go
├── config.yaml              # Connection and safety configuration settings
├── docker-compose.yml       # Local Prometheus integration
├── Dockerfile               # Build configuration
├── prometheus.yml           # Prometheus scrape configuration
├── go.mod
├── go.sum
└── README.md
```

---

## Metrics & Observability 📊

`havoc-engine` exposes a `/metrics` HTTP endpoint (on port 9090) instrumented with `prometheus/client_golang`. 

It tracks the following metrics:
- **`havoc_experiments_total`** (Counter): Total experiments executed, labeled by `experiment_type` (e.g. `kill_pod`) and `result` (`success`, `failed`, `aborted`, `blocked`).
- **`havoc_pod_recovery_seconds`** (Histogram): Time from pod kill to a new pod becoming `Ready` (measured by watching pod status).
- **`havoc_experiment_error_rate`** (Gauge): Current simulated or real error rate percentage used by the safety engine's auto-abort logic.

### Running with Prometheus Locally
You can easily spin up the engine alongside a Prometheus instance using Docker Compose:

```bash
docker-compose up --build -d
```

This starts `havoc-engine` in server mode (`serve` command) and `prometheus` scraping it.
You can view the Prometheus UI at [http://localhost:9091](http://localhost:9091).

To run experiments while the server is up:
```bash
docker-compose exec havoc-engine ./havoc-engine kill-pod --selector=app=demo
```

---

## Prerequisites

- **Go**: 1.21 or higher
- **Kubernetes Cluster**: Local environment like [Minikube](https://minikube.sigs.k8s.io/) or [k3s](https://k3s.io/) / [k3d](https://k3d.io/)
- **kubectl**: Configured to communicate with your local cluster

---

## Configuration

`havoc-engine` uses `config.yaml` to specify connection and safety threshold settings:

```yaml
kubeconfig: "~/.kube/config"
default_namespace: "default"
maxBlastRadiusPercent: 30
abortOnErrorRatePercent: 5.0
```

You can pass a custom config file using the `--config` global flag:

```bash
havoc-engine --config=/path/to/custom-config.yaml <command>
```

---

## Running Unit Tests

`havoc-engine` relies on a clean, testable architecture using Kubernetes fake clientsets (`k8s.io/client-go/kubernetes/fake`). All unit tests (including safety middleware tests) run locally without requiring a live cluster:

```bash
go test -v ./...
```

---

## Local Usage Guide (Minikube / k3s)

### 1. Start your local cluster & deploy a demo app

```bash
# Minikube
minikube start

# Or k3s / k3d
k3d cluster create havoc-cluster

# Deploy a demo workload
kubectl create deployment demo --image=nginx --replicas=4
kubectl get pods -l app=demo
```

---

### 2. Run Chaos Experiments

Build the binary or run directly via `go run ./cmd/havoc-engine`:

#### Build Binary:
```bash
go build -o havoc-engine ./cmd/havoc-engine
```

#### Dry-Run Mode (Simulation):
Test any command safely without executing mutations:

```bash
./havoc-engine kill-pod --namespace=default --selector=app=demo --dry-run
./havoc-engine inject-latency --namespace=default --pod=demo-pod-name --delay=500 --dry-run
```

#### Experiment 1: Kill a Pod
Deletes a random pod matching `app=demo`:

```bash
./havoc-engine kill-pod --namespace=default --selector=app=demo
```

#### Experiment 2: Inject Network Latency
Adds 500ms network latency to `demo-pod` via ephemeral container:

```bash
./havoc-engine inject-latency --namespace=default --pod=demo-pod-name --delay=500
```

#### Experiment 3: Spike CPU Usage
Stresses CPU inside `demo-pod` for 30 seconds:

```bash
./havoc-engine spike-cpu --namespace=default --pod=demo-pod-name --duration=30
```

---

## CLI Reference

```text
Usage:
  havoc-engine [command]

Available Commands:
  inject-latency Inject network delay into a target pod using tc/netem via an ephemeral container
  kill-pod       Delete a random pod matching the selector
  spike-cpu      Stresses CPU inside a target pod using a stress-ng ephemeral container

Flags:
      --config string      Path to config YAML file (default "config.yaml")
      --dry-run            Simulate execution without making actual changes
  -h, --help               help for havoc-engine
  -n, --namespace string   Kubernetes namespace (defaults to config default_namespace)
```
