# Kubernetes Manifests

This folder is organized so Terraform can later apply environment-specific overlays without changing app manifests.

Structure:
- `base/`: reusable manifests (namespace, infra, apps, ingress)
- `overlays/local/`: local/dev overlay
- `overlays/aws/`: AWS overlay (S3 archive backend, removes in-cluster MinIO)

Before applying:
- Build and push images for `collector-service`, `worker-service`, and `query-service`
- Update image names in:
  - `k8s/base/apps/collector/deployment.yaml`
  - `k8s/base/apps/worker/deployment.yaml`
  - `k8s/base/apps/query/deployment.yaml`

Apply local overlay:

```bash
kubectl apply -k k8s/overlays/local
```

Apply AWS overlay:

1) Update `k8s/overlays/aws/patches/configmap-aws.yaml`:
- set `S3_BUCKET` to Terraform output bucket name

2) Apply:

```bash
kubectl apply -k k8s/overlays/aws
```

AWS overlay behavior:
- uses S3 archiving (`WORKER_ARCHIVE_BACKEND=s3`)
- removes in-cluster MinIO resources
- removes in-cluster Postgres resources (apps use RDS via `DB_URL` secret)
- tunes app replicas to 1 each and removes worker HPA for small dev clusters

Current defaults in manifests:
- `raw-events` topic is created with `6` partitions (`k8s/base/infra/redpanda/topic-job.yaml`)
- `worker-service` runs with `3` replicas (`k8s/base/apps/worker/deployment.yaml`)
- `worker-service` has CPU/memory requests+limits and HPA 3-6 replicas (`k8s/base/apps/worker/hpa.yaml`)

HPA note:
- autoscaling needs metrics-server in the cluster.

Notes:
- Worker runs DB migrations from `file://internal/db/migrations`; your worker image must include `internal/db/migrations`.
- Secrets in `base` are dev defaults. Replace with external secret management for non-local environments.
