# Havoc Engine 💥

`havoc-engine` is a lightweight, testable Kubernetes chaos engineering CLI tool built in Go using `client-go`. It allows developers and site reliability engineers to inject controlled failure experiments into Kubernetes clusters to validate resiliency.

---

## Features & Chaos Experiments

1. **`killPod`**: Deletes a random pod matching a target label selector in a given namespace.
2. **`injectLatency`**: Injects network delay (using `tc`/`netem` via an ephemeral container with `NET_ADMIN` capability) into a target pod.
3. **`spikeCPU`**: Stresses CPU cores inside a target pod using a `stress-ng` ephemeral container for a specified duration.

---

## Project Structure

```text
havoc-engine/
├── cmd/
│   └── havoc-engine/
│       └── main.go          # CLI entrypoint and Cobra command wiring
├── internal/
│   ├── config/              # YAML configuration loader (config.yaml)
│   ├── k8sclient/           # Out-of-cluster / In-cluster Kubernetes client initializer
│   └── experiments/         # Chaos experiment implementations and unit tests
│       ├── kill_pod.go
│       ├── kill_pod_test.go
│       ├── inject_latency.go
│       ├── inject_latency_test.go
│       ├── spike_cpu.go
│       └── spike_cpu_test.go
├── config.yaml              # Cluster connection settings (kubeconfig path, default namespace)
├── go.mod
├── go.sum
└── README.md
```

---

## Prerequisites

- **Go**: 1.21 or higher
- **Kubernetes Cluster**: Local environment like [Minikube](https://minikube.sigs.k8s.io/) or [k3s](https://k3s.io/) / [k3d](https://k3d.io/)
- **kubectl**: Configured to communicate with your local cluster

---

## Configuration

`havoc-engine` uses `config.yaml` to specify default cluster connection settings:

```yaml
kubeconfig: "~/.kube/config"
default_namespace: "default"
```

You can pass a custom config file using the `--config` global flag:

```bash
havoc-engine --config=/path/to/custom-config.yaml <command>
```

---

## Running Unit Tests

`havoc-engine` relies on a clean, testable architecture using Kubernetes fake clientsets (`k8s.io/client-go/kubernetes/fake`). All unit tests run locally without requiring a live cluster:

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
kubectl create deployment demo --image=nginx --replicas=3
kubectl get pods -l app=demo
```

---

### 2. Run Chaos Experiments

Build the binary or run directly via `go run ./cmd/havoc-engine`:

#### Build Binary:
```bash
go build -o havoc-engine ./cmd/havoc-engine
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
  -h, --help               help for havoc-engine
  -n, --namespace string   Kubernetes namespace (defaults to config default_namespace)
```
