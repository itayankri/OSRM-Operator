---
name: osrm-dev
description: Expert assistant for the OSRM-Operator Go Kubernetes operator. Use for any development task — bug fixes, new features, reconciliation logic, testing, or GCP deployment.
user-invocable: true
---

⚠️ **PRODUCTION WARNING**: The following GCP clusters are all production environments:
- `gke_af-mapping_europe-west1_eu-maps`
- `gke_af-mapping_europe-west1_maps-prod`
- `gke_af-mapping_asia-northeast1_maps-prod-jp`

You must NEVER perform any read, write, edit, or delete operation on any of these clusters without explicit user permission. Always ask the user for confirmation before executing any command that interacts with these clusters.
---

## Project Overview

Go-based Kubernetes operator using [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime). It manages a single CRD — `OSRMCluster` — which deploys and maintains [OSRM](https://project-osrm.org/) routing engine clusters on Kubernetes.

Per profile, the operator creates and manages:
- `PersistentVolumeClaim` — shared map data storage
- `Job` — downloads and builds OSRM index from a PBF file
- `Deployment` — OSRM worker pods (`osrm-routed`)
- `Service` — internal ClusterIP for the profile
- `HorizontalPodAutoscaler` — CPU-based autoscaling
- `PodDisruptionBudget` — availability guarantee during disruptions
- `CronJob` — optional periodic speed data updates

Plus one shared gateway per cluster:
- `Deployment` — nginx reverse proxy aggregating all profiles
- `Service` — external-facing (LoadBalancer or ClusterIP)
- `ConfigMap` — nginx routing config generated from profile specs

**Key source directories:**
- [api/v1alpha1/](api/v1alpha1/) — CRD types (`OSRMCluster`, `ProfileSpec`, etc.)
- [controllers/](controllers/) — reconciliation logic
- [internal/resource/](internal/resource/) — resource builders
- [internal/status/](internal/status/) — status condition helpers

---

## Key Architecture Concepts

### 7-Phase State Machine

The operator uses a phase-based state machine to manage the cluster lifecycle:

| Phase | Meaning |
|-------|---------|
| `PhaseBuildingMap` | Initial map compilation — PVCs and Jobs created, waiting for Jobs to complete |
| `PhaseDeployingWorkers` | Jobs complete — creating Deployments, Services, HPAs, PDBs, Gateway |
| `PhaseWorkersDeployed` | All workers ready and serving traffic (steady state) |
| `PhaseUpdatingMap` | PBF URL changed — building next-generation PVCs/Jobs in parallel |
| `PhaseRedepoloyingWorkers` | New map ready — updating Deployments to mount new-generation PVCs |
| `PhaseWorkersRedeployed` | Workers running on new map — garbage collect old generation |
| `PhaseDeleting` | CR deletion in progress |

### Blue-Green Map Updates (Zero Downtime)

PVCs and Jobs are named with a generation suffix: `<cluster>-<profile>-<generation>`.

- `activeMapGeneration` = generation currently mounted by running Deployments
- `futureMapGeneration` = highest PVC number (next generation being built)

When PBF URL changes, the operator builds generation N+1 in the background while generation N continues serving traffic. Deployments are only updated after the new map is fully built.

### Resource Builder Pattern

`OSRMResourceBuilder` ([internal/resource/](internal/resource/)) is the central orchestrator. It routes to specialized builders via `ResourceBuildersForPhase()`:

- `DeploymentBuilder` — mounts PVC, runs `osrm-routed --algorithm mld`
- `JobBuilder` — downloads PBF, builds OSRM index, writes to PVC
- `PersistentVolumeClaimBuilder` — storage for compiled map data
- `ServiceBuilder` — ClusterIP per profile, port 80→5000
- `HorizontalPodAutoscalerBuilder` — CPU target 85%
- `PodDisruptionBudgetBuilder` — minAvailable = minReplicas - 1
- `CronJobBuilder` — speed data updates on schedule
- `GatewayDeploymentBuilder` — nginx with envsubst config
- `ConfigMapBuilder` — nginx routing config from profile specs

Each builder implements `Build()` (creates empty object skeleton) and `Update()` (populates spec). Sibling resources are passed into `Update()` to allow cross-resource awareness (e.g. Deployment reads CronJob last-success time for pod annotations).

### Reconciliation Flow

Full flow in [controllers/osrmcluster_controller.go](controllers/osrmcluster_controller.go):

1. Fetch `OSRMCluster` CR
2. Fetch all child resources by label `app.kubernetes.io/name=<cluster-name>`
3. Update status conditions: `Available`, `AllReplicasReady`, `ReconciliationSuccess`
4. First run: add finalizer `osrmcluster.itayankri/finalizer`
5. On deletion: remove finalizer → Kubernetes cleans up children via ownerReferences
6. Check pause annotation `osrm.itayankri/operator.paused=true` → return early if paused
7. Persist last-applied spec as annotation to detect future changes (PBF URL, profile list)
8. **Determine phase**: compare `activeMapGeneration` vs `futureMapGeneration`; check PVC Bound status and Job Complete conditions
9. **Build & apply** resources for current phase via `CreateOrUpdate` with `RetryOnConflict`
10. **Garbage collect**: delete resources for removed profiles; delete old-generation PVCs/Jobs after `PhaseWorkersRedeployed`
11. Update status with reconciliation outcome

### Garbage Collection

The reconciler builds an "expected resources" map from the current spec (all profiles × all resource types). Any existing resource with the cluster label that is NOT in this map gets deleted. Special rules:
- PVCs: always keep active generation; keep future generation during active update phases
- Jobs: same as PVCs

---

## Resource Naming Conventions

| Resource | Name Pattern | Example |
|----------|-------------|---------|
| Deployment (profile) | `<cluster>-<profile>` | `my-osrm-car` |
| Service (profile) | `<cluster>-<profile>` | `my-osrm-car` |
| HPA | `<cluster>-<profile>` | `my-osrm-car` |
| PDB | `<cluster>-<profile>` | `my-osrm-car` |
| PVC | `<cluster>-<profile>-<gen>` | `my-osrm-car-1` |
| Job | `<cluster>-<profile>-map-builder` | `my-osrm-car-map-builder` |
| CronJob | `<cluster>-<profile>-speed-updates` | `my-osrm-car-speed-updates` |
| Gateway Deployment | `<cluster>` | `my-osrm` |
| Gateway Service | `<cluster>` | `my-osrm` |
| Gateway ConfigMap | `<cluster>` | `my-osrm` |

**Labels on all child resources:**
```
app.kubernetes.io/name: <cluster-name>
app.kubernetes.io/part-of: osrmcluster
app.kubernetes.io/component: gateway | <profile-name>
```

---

## GCP / Cluster Context

**Production contexts — all require explicit permission before any operation:**

| Context | Region |
|---------|--------|
| `gke_af-mapping_europe-west1_eu-maps` | Europe West 1 (default) |
| `gke_af-mapping_europe-west1_maps-prod` | Europe West 1 |
| `gke_af-mapping_asia-northeast1_maps-prod-jp` | Asia Northeast 1 (Japan) |

```bash
kubectl config use-context gke_af-mapping_europe-west1_eu-maps
```

**Useful read-only commands for debugging:**
```bash
# Check cluster status
kubectl get osrmcluster -A
kubectl describe osrmcluster <name>

# Check operator pod
kubectl get pods -n osrm-system
kubectl logs -n osrm-system deploy/osrm-controller-manager -c manager --tail=200 -f

# Check profile resources
kubectl get deploy,svc,hpa,pdb -l app.kubernetes.io/name=<cluster-name>
kubectl get pvc,jobs -l app.kubernetes.io/name=<cluster-name>

# Check map building progress
kubectl logs job/<cluster>-<profile>-map-builder

# Check worker pod health
kubectl get pods -l app.kubernetes.io/name=<cluster-name>
kubectl logs deploy/<cluster>-<profile>

# Check gateway config
kubectl get configmap <cluster> -o yaml
```

---

## Developer Workflow

```bash
# After editing api/v1alpha1/ types — regenerate CRDs and DeepCopy methods
make manifests generate

# Format and lint
make fmt vet

# Run fast unit tests (~5 min, no cluster needed)
make unit-test

# Run integration tests (~15 min, needs envtest)
make integration-test

# Run full e2e tests (~30 min)
make e2e-test

# Build operator binary
make build

# Build and push container image
make docker-build IMG=<registry>/osrm-operator:<tag>
make docker-push  IMG=<registry>/osrm-operator:<tag>

# Deploy to cluster in ~/.kube/config
make install                              # install CRDs only
make deploy IMG=<registry>/osrm-operator:<tag>  # deploy operator
make undeploy                             # remove operator
make uninstall                            # remove CRDs
```

---

## OSRMCluster CR Quick Reference

See [examples.md](examples.md) for a full annotated YAML covering all available fields.

