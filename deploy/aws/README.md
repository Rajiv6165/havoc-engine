# Havoc Engine: AWS Production & Public Demo Deployment Guide

This guide details step-by-step instructions for running the complete Havoc Engine stack on a single small Amazon EC2 instance using **k3s**, **Docker Compose**, and **Caddy** as a reverse proxy with automated Let's Encrypt HTTPS certificates.

---

## ⚠️ Critical Operational Warning

> **The Havoc Dashboard's "Run Experiment" buttons trigger actual mutations against the connected Kubernetes cluster!**
>
> In a live cluster, triggering `kill-pod`, `inject-latency`, or `spike-cpu` deletes real pods or attaches ephemeral debug containers. If your dashboard is exposed publicly on the internet without safeguards, **any visitor could disrupt workloads or degrade cluster performance**.
>
> For public demonstrations, your deployment **MUST** either:
> 1. Run in **Read-Only Demo Mode** (`DEMO_READ_ONLY=true`), which disables all chaos action buttons and allows visitors to inspect status, topology, score history, and postmortems safely; **OR**
> 2. Require authentication using a strong `HAVOC_API_TOKEN` and restrict network access via AWS Security Groups or VPN/allowlists.

---

## 🏛 Architecture on EC2

```
                       Internet
                          │
                   HTTPS (80 / 443)
                          ▼
             ┌─────────────────────────┐
             │   Caddy Reverse Proxy   │ (Automatic TLS)
             └────────────┬────────────┘
                          │
            ┌─────────────┴─────────────┐
            │                           │
            ▼                           ▼
   ┌─────────────────┐       ┌─────────────────┐
   │ Havoc Dashboard │       │  WebSocket (:91) │
   │  (Next.js :3000)│       │  Event Stream   │
   └────────┬────────┘       └────────▲────────┘
            │ (Internal Server-Side    │
            │  API Proxy Only)         │
            ▼                          │
   ┌─────────────────┐                 │
   │  Havoc Engine   ├─────────────────┘
   │  (Go API :8080) │
   └────────┬────────┘
            │
            ├──────────────► PostgreSQL (:5432 - Internal Network Only)
            ├──────────────► Prometheus (:9090 - Internal Network Only)
            ▼
   ┌─────────────────┐
   │   k3s Cluster   │ (Host Kubernetes: demo microservices)
   └─────────────────┘
```

- **Publicly Accessible**: Caddy reverse proxy on ports `80` (HTTP redirect) and `443` (HTTPS).
- **Protected Internally**:
  - `havoc-dashboard`: Port 3000 exposed only within the Docker network to Caddy.
  - `havoc-engine`: API (:8080) and metrics (:9090) are **never exposed to the public internet**; only reachable by the dashboard's server-side proxy handlers.
  - `postgres` (:5432) and `prometheus` (:9090): Internal Docker network only.

---

## 📋 Prerequisites

- An active **AWS Account** with permissions to launch EC2 instances and configure Security Groups.
- A **Domain Name** (e.g. `havoc.yourdomain.com`) with an `A` record pointing to your EC2 Elastic IP or public IP address.
- An SSH key pair created in AWS (e.g. `havoc-key.pem`).

---

## 🚀 Step-by-Step Deployment

### Step 1: Launch an EC2 Instance

1. Navigate to the **Amazon EC2 Console** and click **Launch Instance**.
2. **Name**: `havoc-engine-demo`
3. **AMI**: Ubuntu Server 22.04 LTS (HVM), SSD Volume Type (64-bit x86).
4. **Instance Type**: `t3.small` (2 vCPU, 2 GiB RAM) for minimal testing, or `t3.medium` (2 vCPU, 4 GiB RAM) recommended.
5. **Key Pair**: Select or create your key pair (`havoc-key.pem`).
6. **Storage**: 20 GiB gp3 root volume.

### Step 2: Configure Security Group (Firewall)

Under **Network Settings**, create a Security Group with strict ingress rules:

| Type | Port Range | Source | Purpose |
|---|---|---|---|
| **SSH** | `22` | **My IP** (`x.x.x.x/32`) | Secure terminal access (NEVER use `0.0.0.0/0`) |
| **HTTP** | `80` | `0.0.0.0/0` | Let's Encrypt HTTP-01 challenge & redirect to HTTPS |
| **HTTPS** | `443` | `0.0.0.0/0` | Public dashboard access via TLS |

> **All other ports (8080, 9090, 9091, 5432) must remain CLOSED to the internet.**

Click **Launch Instance**.

### Step 3: Connect and Install Dependencies

Allocate an **Elastic IP** in AWS and associate it with your instance. Point your DNS `A` record to this Elastic IP.

Connect via SSH:
```bash
ssh -i havoc-key.pem ubuntu@<YOUR-EC2-ELASTIC-IP>
```

Update system packages and install Docker and git:
```bash
sudo apt-get update && sudo apt-get upgrade -y
sudo apt-get install -y curl git apt-transport-https ca-certificates gnupg lsb-release

# Install Docker
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Grant ubuntu user docker access
sudo usermod -aG docker ubuntu
newgrp docker
```

### Step 4: Install and Configure k3s

Install k3s without Traefik (since Caddy serves as our reverse proxy):
```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh -

# Verify k3s cluster is running
sudo kubectl get nodes

# Configure kubeconfig read permissions for Havoc Engine container
sudo chmod 644 /etc/rancher/k3s/k3s.yaml
```

### Step 5: Deploy Demo Target Microservices

Clone the repository and deploy the demo microservices into the isolated `demo` namespace:
```bash
git clone https://github.com/Rajiv6165/havoc-engine.git
cd havoc-engine

# Create demo namespace
sudo kubectl create namespace demo

# Deploy frontend, api, db, and queue deployments
sudo kubectl apply -f deploy/demo/app.yaml -n demo

# Verify pods are running
sudo kubectl get pods -n demo
```

### Step 6: Configure Environment Secrets

Create an environment file `.env` for production:
```bash
cat << 'EOF' > .env
# Domain configuration for Caddy automated HTTPS
DOMAIN=havoc.yourdomain.com

# Enable Read-Only mode for public demo visitors
DEMO_READ_ONLY=true

# Safety namespace allowlist - strictly limits chaos to demo namespace
HAVOC_ALLOWED_NAMESPACES=demo

# Shared API token protecting mutable endpoints
HAVOC_API_TOKEN=generate-a-secure-random-token-here-32chars

# PostgreSQL password
POSTGRES_PASSWORD=generate-a-secure-db-password

# Optional Anthropic Claude API key for postmortem report generation
ANTHROPIC_API_KEY=sk-ant-api03-...

# Public WebSocket URL for dashboard browser connections
NEXT_PUBLIC_WS_URL=wss://havoc.yourdomain.com/ws
EOF
```

### Step 7: Launch Havoc Engine Stack

Start the production stack with `docker-compose.prod.yml`:
```bash
docker compose -f docker-compose.prod.yml up -d --build
```

Verify all containers are healthy:
```bash
docker compose -f docker-compose.prod.yml ps
```

Check Caddy logs to confirm Let's Encrypt certificate issuance:
```bash
docker compose -f docker-compose.prod.yml logs caddy
```

### Step 8: Seed Initial Baseline History

Populate the dashboard with baseline incident history and AI postmortems:
```bash
docker compose -f docker-compose.prod.yml exec havoc-engine havoc-engine seed-demo
```

Open `https://havoc.yourdomain.com` in your browser!

---

## 🔒 Security Checklist for a Public Demo

Before sharing your public URL, verify each item on this checklist:

- [ ] **Read-Only Demo Mode Enabled**:
  Ensure `DEMO_READ_ONLY=true` is set in `.env`. Verify in the browser that the **Kill Pod**, **Inject Latency**, and **Spike CPU** buttons are disabled with the tooltip *"Run locally to inject real chaos"*.
- [ ] **Strict Namespace Allowlist**:
  Verify `HAVOC_ALLOWED_NAMESPACES=demo`. Ensure system namespaces (`kube-system`, `kube-public`, `kube-node-lease`) cannot be targeted under any circumstances.
- [ ] **API Token Configured**:
  Set `HAVOC_API_TOKEN` to a cryptographically strong secret. Even if a user attempts direct curl requests to the internal API, non-read requests are blocked with `401 Unauthorized`.
- [ ] **Engine and Database Unexposed**:
  Run `sudo netstat -tlpn` or `ss -tlpn` on the EC2 host. Verify that ports `8080`, `9090`, `9091`, and `5432` are **NOT** bound to `0.0.0.0` on the host. Only Caddy (`:80` and `:443`) and SSH (`:22`) should be listening.
- [ ] **AWS Security Group Restricted**:
  Confirm port `22` is restricted exclusively to your own public IP address.
- [ ] **AWS Billing Alarm**:
  Set an AWS CloudWatch billing budget alarm (e.g. \$10 threshold) in the AWS Billing Console to notify you if unexpected charges accumulate.

---

## 🧹 Teardown & Cleanup

When your demo period concludes, follow these steps to completely tear down all resources and prevent ongoing AWS charges:

1. **Stop and Remove Docker Containers and Volumes**:
   ```bash
   docker compose -f docker-compose.prod.yml down -v
   ```

2. **Uninstall k3s**:
   ```bash
   /usr/local/bin/k3s-uninstall.sh
   ```

3. **Release Elastic IP & Terminate EC2 Instance**:
   - In the **Amazon EC2 Console**, select `havoc-engine-demo` -> **Instance State** -> **Terminate Instance**.
   - Navigate to **Network & Security** -> **Elastic IPs** -> select your Elastic IP -> **Actions** -> **Release Elastic IP address**.
   - Navigate to **Elastic Block Store** -> **Volumes** -> confirm that the associated EBS volume has been deleted.
