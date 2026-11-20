# Demo Microservices Variants

This directory provides two deployment variants for the Havoc Engine demo application in the `demo` namespace:

1. **`app-baseline.yaml` (Deliberately Fragile)**:
   - 1 replica for `frontend` and `api`.
   - No `PodDisruptionBudget` (PDB).
   - No resource requests or limits (unbounded CPU/memory consumption).
   - No liveness probes.
   - Intended to demonstrate how pod failures or resource exhaustion directly result in service downtime and low resilience scores.

2. **`app-hardened.yaml` (Hardened for Resilience)**:
   - 3 replicas for `frontend` and `api`.
   - `PodDisruptionBudget` (`minAvailable: 2`) preventing simultaneous outages during voluntary disruptions.
   - Comprehensive `readinessProbe` and `livenessProbe` definitions to detect and isolate unhealthy containers rapidly.
   - Explicit container `resources.requests` and `resources.limits` preventing CPU starvation and OOM cascades.
   - `topologySpreadConstraints` with `whenUnsatisfiable: ScheduleAnyway` ensuring balanced pod placement across cluster nodes.

Both manifests define identical Service and Deployment names (`frontend`, `api`, `db`, `queue`) in the `demo` namespace, allowing seamless switching.

---

## Switching Between Variants

### 1. Apply Baseline (Fragile)
To deploy or revert to the fragile single-replica baseline:

```bash
# Apply the baseline manifest
kubectl apply -f deploy/demo/app-baseline.yaml -n demo

# Remove any PodDisruptionBudgets left over from hardened mode
kubectl delete pdb -n demo --all --ignore-not-found
```

### 2. Apply Hardened (Resilient)
To deploy the hardened multi-replica configuration:

```bash
kubectl apply -f deploy/demo/app-hardened.yaml -n demo
```

### 3. Verify Current Deployment
Check running pods, replica counts, and active disruption budgets:

```bash
kubectl get pods,deployments,pdb -n demo
```

---

## Measuring Real Availability via Probing

To measure HTTP availability and downtime during chaos experiments, expose the demo `api` (or `frontend`) service:

### Option A: Port-forwarding (Recommended for local clusters)
Forward the `api` service to your local machine:
```bash
kubectl port-forward svc/api 8080:80 -n demo
```

Then supply `--probe-url` to any experiment command:
```bash
# Example kill-pod experiment with active HTTP availability probing
havoc-engine kill-pod -s app=api -n demo --probe-url http://localhost:8080/healthz --label baseline
```

### Option B: NodePort
If your cluster node port is accessible:
```bash
# Look up the NodePort assigned to the service
kubectl get svc api -n demo

# Target the node's external address and assigned port
havoc-engine kill-pod -s app=api -n demo --probe-url http://<NODE_IP>:<NODE_PORT>/healthz --label hardened
```

