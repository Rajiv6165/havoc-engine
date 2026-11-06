#!/usr/bin/env bash
set -euo pipefail

echo "=========================================================="
echo "           🚀 Havoc Engine Demo Environment Setup         "
echo "=========================================================="

# 1. Check prerequisites
echo "==> Checking prerequisites..."

# Docker check
if ! command -v docker >/dev/null 2>&1; then
    echo "❌ Error: 'docker' is not installed or not in PATH."
    exit 1
fi

if ! docker info >/dev/null 2>&1; then
    echo "❌ Error: Docker daemon is not running. Please start Docker."
    exit 1
fi
echo "  ✓ Docker is available and running."

# Kubectl check
if ! command -v kubectl >/dev/null 2>&1; then
    echo "❌ Error: 'kubectl' is not installed or not in PATH."
    exit 1
fi
echo "  ✓ kubectl is available."

# Minikube or K3d check
CLUSTER_CMD=""
if command -v minikube >/dev/null 2>&1; then
    CLUSTER_CMD="minikube"
    echo "  ✓ minikube detected."
elif command -v k3d >/dev/null 2>&1; then
    CLUSTER_CMD="k3d"
    echo "  ✓ k3d detected."
else
    # Check if a kubernetes cluster is already reachable
    if kubectl cluster-info >/dev/null 2>&1; then
        echo "  ✓ Existing Kubernetes cluster detected via kubectl."
        CLUSTER_CMD="existing"
    else
        echo "❌ Error: Neither 'minikube' nor 'k3d' found, and no active Kubernetes cluster is reachable."
        echo "   Please install minikube (https://minikube.sigs.k8s.io/) or k3d (https://k3d.io/)."
        exit 1
    fi
fi

# 2. Start local cluster if needed
echo "==> Ensuring Kubernetes cluster is running..."
if [ "$CLUSTER_CMD" = "minikube" ]; then
    if ! minikube status >/dev/null 2>&1; then
        echo "Starting minikube cluster..."
        minikube start
    else
        echo "minikube cluster is already running."
    fi
elif [ "$CLUSTER_CMD" = "k3d" ]; then
    if ! k3d cluster list 2>/dev/null | grep -q "havoc-demo"; then
        echo "Creating k3d cluster 'havoc-demo'..."
        k3d cluster create havoc-demo
    else
        echo "k3d cluster 'havoc-demo' already exists."
    fi
fi

# Verify cluster connectivity
echo "Verifying cluster access..."
kubectl cluster-info >/dev/null 2>&1 || {
    echo "❌ Error: Unable to communicate with Kubernetes cluster."
    exit 1
}
echo "  ✓ Connected to Kubernetes cluster."

# 3. Create the 'demo' namespace
echo "==> Creating 'demo' namespace..."
kubectl create namespace demo --dry-run=client -o yaml | kubectl apply -f -

# 4. Apply deploy/demo/app.yaml
echo "==> Deploying demo microservices (frontend, api, db, queue)..."
kubectl apply -f deploy/demo/app.yaml -n demo

# 5. Run docker-compose up --build -d
echo "==> Starting Havoc Engine stack via Docker Compose..."
if docker compose version >/dev/null 2>&1; then
    docker compose up --build -d
elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose up --build -d
else
    echo "❌ Error: neither 'docker compose' nor 'docker-compose' is available."
    exit 1
fi

# 6. Wait until /api/status responds
echo "==> Waiting for Havoc Engine API to become ready (http://localhost:8080/api/status)..."
MAX_RETRIES=60
COUNT=0
READY=0
while [ $COUNT -lt $MAX_RETRIES ]; do
    if curl -s -f "http://localhost:8080/api/status" >/dev/null 2>&1; then
        READY=1
        break
    fi
    sleep 2
    COUNT=$((COUNT + 1))
    echo -n "."
done
echo ""

if [ $READY -ne 1 ]; then
    echo "⚠️ Warning: Timed out waiting for API server /api/status to respond."
else
    echo "  ✓ Havoc Engine API is healthy and responding!"
fi

# 7. Seed demo environment
echo "==> Seeding baseline chaos history and AI postmortems..."
if docker compose version >/dev/null 2>&1; then
    docker compose exec -T havoc-engine havoc-engine seed-demo || echo "Note: docker-compose seed-demo step skipped"
elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose exec -T havoc-engine havoc-engine seed-demo || echo "Note: docker-compose seed-demo step skipped"
elif command -v go >/dev/null 2>&1; then
    go run ./cmd/havoc-engine seed-demo || true
fi

# 8. Print dashboard URL and suggested clicks
echo ""
echo "=========================================================="
echo "  🎉 Havoc Engine Demo Environment is Ready!"
echo "=========================================================="
echo ""
echo "  📊 Dashboard URL: http://localhost:3000"
echo "  ⚙️ API Server:   http://localhost:8080"
echo "  📈 Prometheus:   http://localhost:9092"
echo ""
echo "Suggested 3 clicks to explore the demo:"
echo "  1. Explore Service Topology:"
echo "     Click the 'Topology' tab to inspect real-time inter-service dependencies"
echo "     and live pod health for frontend, api, db, and queue."
echo ""
echo "  2. Inject Live Chaos:"
echo "     Click 'Kill Pod' or 'Inject Latency' on the frontend service node."
echo "     Watch real-time WebSocket events stream in the Live Incident Log"
echo "     as the safety engine watches pod recovery."
echo ""
echo "  3. Inspect AI Postmortems & Resilience Scores:"
echo "     Click any completed experiment in the 'Experiment History' panel to open"
echo "     the AI-generated root cause analysis, SLA impact, and remediation report."
echo ""
echo "=========================================================="

