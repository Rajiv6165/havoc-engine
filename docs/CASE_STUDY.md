# Case Study: Resilience Comparison (Baseline vs. Hardened)

This case study measures and compares the resilience characteristics of a Kubernetes microservices application under intentional failure injection, comparing an unoptimized baseline deployment against a hardened deployment architecture using **Havoc Engine**.

---

## 🎯 Goal

The objective of this evaluation is to empirically measure how standard Kubernetes resilience best practices impact service availability, recovery latency, and error rates during chaos events:
1. Does multi-replica redundancy with PodDisruptionBudgets eliminate downtime during pod terminations?
2. How do explicit CPU/memory limits and readiness probes affect blast radius propagation?
3. How much does a service's quantitative Resilience Score improve between naive and hardened configurations?

> [!NOTE]
> All figures in this document are generated from real, stored experiment data using `havoc-engine compare --format=markdown`. No numbers are simulated or fabricated.

---

## 🛠️ Setup

### Cluster Environment
- **Cluster**: Local Kubernetes (`minikube` / `k3d`) or single-node EC2 instance (`k3s`).
- **Control Plane**: Havoc Engine Go backend with PostgreSQL 15, Prometheus 2.45, and Next.js 14 Dashboard.
- **Namespace**: `demo`

### Application Variants

The test application consists of four interconnected microservices: `frontend` ➡️ `api` ➡️ `db` + `queue`.

1. **Baseline Variant (`deploy/demo/app-baseline.yaml`)**:
   - Single replica (`replicas: 1`) for all services.
   - No `PodDisruptionBudget` (PDB).
   - No container resource requests or limits.
   - Minimal or absent liveness/readiness probes.
   - No placement constraints.

2. **Hardened Variant (`deploy/demo/app-hardened.yaml`)**:
   - Multi-replica redundancy (`replicas: 3`) for `frontend` and `api`.
   - `PodDisruptionBudget` (`minAvailable: 2`) configured for critical components.
   - Explicit `readinessProbe` and `livenessProbe` definitions with short evaluation intervals.
   - Enforced CPU and memory requests (`50m` / `64Mi`) and limits (`200m` / `128Mi`).
   - `topologySpreadConstraints` with `whenUnsatisfiable: ScheduleAnyway` ensuring balanced pod distribution across nodes.

---

## 🧪 Method

### Experiment Workflow
Each test cycle applies one variant, triggers a series of randomized failure injections, and records metrics to PostgreSQL:
- **Actions Tested**: Pod terminations (`kill-pod`), network latency injection (`inject-latency`), and CPU contention (`spike-cpu`).
- **Iteration Count**: Minimum of 5 runs per variant (recommended: 5–10) to obtain valid averages and 95th percentile (P95) recovery times.
- **Labeling**: Every experiment run is tagged using `--label=baseline` or `--label=hardened`.

### Reproduction Commands

```bash
# 1. Deploy baseline configuration
kubectl apply -f deploy/demo/app-baseline.yaml -n demo
kubectl delete pdb -n demo --all --ignore-not-found

# 2. Run baseline chaos iterations
for i in {1..5}; do
  havoc-engine kill-pod -n demo -s app=frontend --label=baseline
  sleep 5
done

# 3. Deploy hardened configuration
kubectl apply -f deploy/demo/app-hardened.yaml -n demo

# 4. Run hardened chaos iterations
for i in {1..5}; do
  havoc-engine kill-pod -n demo -s app=frontend --label=hardened
  sleep 5
done

# 5. Output comparison report in Markdown
havoc-engine compare --a=baseline --b=hardened --format=markdown
```

---

## 📊 Results

<!-- TODO: fill from real runs using 'havoc-engine compare --a=baseline --b=hardened --format=markdown' -->

| Metric | Baseline (`baseline`) | Hardened (`hardened`) | Difference (Δ) |
|---|---|---|---|
| **Run Count** | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` |
| **Avg Resilience Score** | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` |
| **Avg Recovery Time** | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` |
| **P95 Recovery Time** | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` |
| **Safety Interventions** | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` | `<!-- TODO: fill from real runs -->` |

*(Note: P95 recovery times are computed when 5 or more runs exist per label.)*

---

## 🔍 Findings

<!-- TODO: fill from real runs -->
- **Service Availability**: `<!-- TODO: fill from real runs: describe how 3 replicas + PDB maintained active endpoints while 1 replica caused total drops -->`
- **Recovery Latency**: `<!-- TODO: fill from real runs: compare pod reschedule and initialization times between configurations -->`
- **Error Rates & Interventions**: `<!-- TODO: fill from real runs: note whether Prometheus error rate auto-abort thresholds were triggered in baseline vs hardened -->`

---

## 🛠️ What I Changed

To transition from the fragile baseline to the resilient hardened architecture, the following configuration patterns were implemented:

1. **Horizontal Redundancy**:
   Scaled `replicas` from 1 to 3 for user-facing services (`frontend`, `api`), ensuring surviving pods handle live traffic while replacements spin up.
2. **PodDisruptionBudgets (PDB)**:
   Added `minAvailable: 2` for `frontend-pdb` and `api-pdb` to guard against concurrent pod eviction during voluntary maintenance or rolling updates.
3. **Health Probes**:
   Defined tuned `readinessProbe` (initial delay 1s, period 2s) and `livenessProbe` (initial delay 2s, period 5s) to immediately remove unready pods from Service endpoints and restart crashed processes.
4. **Resource Quotas**:
   Set explicit CPU and memory bounds (`resources.requests` and `resources.limits`) to protect against noisy neighbors and memory leaks.
5. **Topology Spread**:
   Added `topologySpreadConstraints` to ensure pod replicas are distributed across different nodes/hosts rather than co-locating on a single point of failure.

---

## ⚠️ Limitations

1. **Sample Size**:
   A minimum of 3 runs is enforced by Havoc Engine before presenting comparison summaries, with 5+ runs required for P95 percentiles. Statistical significance increases with sample size (recommended: 10–20 iterations per test).
2. **Local Cluster vs. Production Cloud**:
   Local environments (e.g. Minikube / K3d / single-node K3s) share physical node resources and kernel schedulers. Multi-zone cloud clusters (EKS / GKE) introduce realistic cross-AZ network latencies and VM provisioning delays.
3. **Pod Readiness vs. User Downtime**:
   Recovery time is measured from Kubernetes Watch stream events until the container satisfies `Ready=True`. This measures infrastructure recovery time rather than end-to-end client-side request draining or CDN edge retry dynamics.
