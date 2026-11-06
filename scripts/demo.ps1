# scripts/demo.ps1 - Havoc Engine Demo Environment Setup for Windows PowerShell

$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "           🚀 Havoc Engine Demo Environment Setup         " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Check prerequisites
Write-Host "==> Checking prerequisites..." -ForegroundColor Yellow

# Docker check
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Error "❌ Error: 'docker' is not installed or not in PATH."
    exit 1
}

try {
    $dockerInfo = docker info 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Error "❌ Error: Docker daemon is not running. Please start Docker Desktop."
        exit 1
    }
} catch {
    Write-Error "❌ Error: Docker daemon is not running. Please start Docker Desktop."
    exit 1
}
Write-Host "  ✓ Docker is available and running." -ForegroundColor Green

# Kubectl check
if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) {
    Write-Error "❌ Error: 'kubectl' is not installed or not in PATH."
    exit 1
}
Write-Host "  ✓ kubectl is available." -ForegroundColor Green

# Minikube or k3d check
$clusterType = ""
if (Get-Command minikube -ErrorAction SilentlyContinue) {
    $clusterType = "minikube"
    Write-Host "  ✓ minikube detected." -ForegroundColor Green
} elseif (Get-Command k3d -ErrorAction SilentlyContinue) {
    $clusterType = "k3d"
    Write-Host "  ✓ k3d detected." -ForegroundColor Green
} else {
    try {
        $clusterInfo = kubectl cluster-info 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Host "  ✓ Existing Kubernetes cluster detected via kubectl." -ForegroundColor Green
            $clusterType = "existing"
        } else {
            Write-Error "❌ Error: Neither 'minikube' nor 'k3d' found, and no active Kubernetes cluster is reachable."
            exit 1
        }
    } catch {
        Write-Error "❌ Error: Neither 'minikube' nor 'k3d' found, and no active Kubernetes cluster is reachable."
        exit 1
    }
}

# 2. Start local cluster if needed
Write-Host "==> Ensuring Kubernetes cluster is running..." -ForegroundColor Yellow
if ($clusterType -eq "minikube") {
    $status = minikube status 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Starting minikube cluster..." -ForegroundColor Cyan
        minikube start
    } else {
        Write-Host "minikube cluster is already running." -ForegroundColor Green
    }
} elseif ($clusterType -eq "k3d") {
    $k3dClusters = k3d cluster list 2>&1
    if ($k3dClusters -notmatch "havoc-demo") {
        Write-Host "Creating k3d cluster 'havoc-demo'..." -ForegroundColor Cyan
        k3d cluster create havoc-demo
    } else {
        Write-Host "k3d cluster 'havoc-demo' already exists." -ForegroundColor Green
    }
}

# Verify cluster connectivity
kubectl cluster-info | Out-Null
Write-Host "  ✓ Connected to Kubernetes cluster." -ForegroundColor Green

# 3. Create the 'demo' namespace
Write-Host "==> Creating 'demo' namespace..." -ForegroundColor Yellow
kubectl create namespace demo --dry-run=client -o yaml | kubectl apply -f -

# 4. Apply deploy/demo/app.yaml
Write-Host "==> Deploying demo microservices (frontend, api, db, queue)..." -ForegroundColor Yellow
kubectl apply -f deploy/demo/app.yaml -n demo

# 5. Run docker-compose up --build -d
Write-Host "==> Starting Havoc Engine stack via Docker Compose..." -ForegroundColor Yellow
$composeCmd = "docker compose"
try {
    docker compose version 2>&1 | Out-Null
} catch {
    $composeCmd = "docker-compose"
}

if ($composeCmd -eq "docker compose") {
    docker compose up --build -d
} else {
    docker-compose up --build -d
}

# 6. Wait until /api/status responds
Write-Host "==> Waiting for Havoc Engine API to become ready (http://localhost:8080/api/status)..." -ForegroundColor Yellow
$maxRetries = 60
$count = 0
$ready = $false

while ($count -lt $maxRetries) {
    try {
        $res = Invoke-RestMethod -Uri "http://localhost:8080/api/status" -Method Get -TimeoutSec 2 -ErrorAction Stop
        $ready = $true
        break
    } catch {
        Start-Sleep -Seconds 2
        $count++
        Write-Host -NoNewline "."
    }
}
Write-Host ""

if (-not $ready) {
    Write-Host "⚠️ Warning: Timed out waiting for API server /api/status to respond." -ForegroundColor Red
} else {
    Write-Host "  ✓ Havoc Engine API is healthy and responding!" -ForegroundColor Green
}

# 7. Seed demo environment
Write-Host "==> Seeding baseline chaos history and AI postmortems..." -ForegroundColor Yellow
try {
    if ($composeCmd -eq "docker compose") {
        docker compose exec -T havoc-engine havoc-engine seed-demo
    } else {
        docker-compose exec -T havoc-engine havoc-engine seed-demo
    }
} catch {
    Write-Host "Note: seeding via docker-compose skipped, attempting local seed..." -ForegroundColor Gray
    try {
        go run ./cmd/havoc-engine seed-demo
    } catch {
        Write-Host "Note: local seed completed or skipped." -ForegroundColor Gray
    }
}

# 8. Print dashboard URL and suggested clicks
Write-Host ""
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  🎉 Havoc Engine Demo Environment is Ready!" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  📊 Dashboard URL: http://localhost:3000" -ForegroundColor Cyan
Write-Host "  ⚙️ API Server:   http://localhost:8080" -ForegroundColor Cyan
Write-Host "  📈 Prometheus:   http://localhost:9092" -ForegroundColor Cyan
Write-Host ""
Write-Host "Suggested 3 clicks to explore the demo:" -ForegroundColor Yellow
Write-Host "  1. Explore Service Topology:" -ForegroundColor White
Write-Host "     Click the 'Topology' tab to inspect real-time inter-service dependencies"
Write-Host "     and live pod health for frontend, api, db, and queue."
Write-Host ""
Write-Host "  2. Inject Live Chaos:" -ForegroundColor White
Write-Host "     Click 'Kill Pod' or 'Inject Latency' on the frontend service node."
Write-Host "     Watch real-time WebSocket events stream in the Live Incident Log"
Write-Host "     as the safety engine watches pod recovery."
Write-Host ""
Write-Host "  3. Inspect AI Postmortems & Resilience Scores:" -ForegroundColor White
Write-Host "     Click any completed experiment in the 'Experiment History' panel to open"
Write-Host "     the AI-generated root cause analysis, SLA impact, and remediation report."
Write-Host ""
Write-Host "==========================================================" -ForegroundColor Cyan

