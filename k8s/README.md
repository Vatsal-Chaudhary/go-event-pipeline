# Kubernetes Manifests

Kubernetes manifests are split to keep app definitions reusable across local and AWS.

## Layout

- `base/`: shared manifests (namespace, infra, apps, ingress)
- `overlays/local/`: local/dev overlay
- `overlays/aws/`: AWS runtime overlay

## Local apply

```bash
kubectl apply -k k8s/overlays/local
```

## AWS apply

1. Ensure `event-pipeline` namespace exists and `event-pipeline-secrets` is created.
2. Ensure `event-pipeline-config` has runtime values (`S3_BUCKET`, `REDIS_ADDR`, etc.).
3. Apply overlay:

```bash
kubectl apply -k k8s/overlays/aws
```

## AWS overlay behavior

- sets worker archive backend to S3
- removes in-cluster MinIO resources
- removes in-cluster Postgres resources (apps use RDS via `DB_URL`)
- sets collector/worker/query replicas to 1 for small dev clusters
- removes worker HPA in AWS overlay to avoid noisy autoscaling dependencies

## Validation commands

```bash
kubectl -n event-pipeline get pods
kubectl -n event-pipeline rollout status deploy/collector-service
kubectl -n event-pipeline rollout status deploy/worker-service
kubectl -n event-pipeline rollout status deploy/query-service
```

## Access for testing

If ingress controller is not installed, use port-forward:

```bash
kubectl -n event-pipeline port-forward svc/collector-service 3000:80
kubectl -n event-pipeline port-forward svc/query-service 3002:80
```

Then run project tests from repo root.

## Notes

- Worker DB migrations run at startup; worker image must include `internal/db/migrations`.
- Base secrets are dev defaults only; for non-local environments, provide secrets at deploy time.
