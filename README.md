# Go Event Pipeline

Simple event pipeline using Go, Redpanda (Kafka), Redis, PostgreSQL, and MinIO.

## Flow

`collector-service` -> Kafka `raw-events` -> `worker-service`

Worker does:
- archive `raw`
- dedupe by `event_id`
- fraud check by IP threshold (in-process Redis logic)
- archive `duplicate` / `fraud` / `accepted`
- persist only accepted events to PostgreSQL

## Project Highlights

- Event-driven processing with clear stage boundaries: ingest, dedupe, fraud scoring, archive, persistence.
- Idempotency-first worker path using Redis dedupe keying on `event_id`.
- Cost-conscious archival design: batched NDJSON + gzip objects with status prefixes for retention policies.
- Operational focus: graceful shutdown flushing, background batch uploader, and strict end-to-end smoke testing.
- Kubernetes-ready manifests with resource requests/limits, HPA, and topic/bootstrap jobs for reproducible bring-up.

## Tradeoffs

- Fraud scoring is in-process (worker + Redis) for lower latency and simpler operations; this reduces independent deployability compared to a separate fraud service.
- Redis-backed counters use TTL windows (simple and fast) but are approximate under highly distributed traffic compared to full sliding-window analytics.
- Worker is scaled with Kafka consumer groups; throughput is bounded by topic partitions.
- K8s local stack runs stateful infra in-cluster for demo simplicity; production should prefer managed services (RDS, ElastiCache, S3, MSK).
- Current reliability posture is pragmatic for a portfolio project; production hardening would add DLQ/replay pipelines, stronger observability, and tighter IAM/secret management.

Archive files are batched NDJSON + gzip in MinIO:
- `raw/YYYY/MM/DD/HH/batch_*.ndjson.gz`
- `duplicate/YYYY/MM/DD/HH/batch_*.ndjson.gz`
- `fraud/YYYY/MM/DD/HH/batch_*.ndjson.gz`
- `accepted/YYYY/MM/DD/HH/batch_*.ndjson.gz`

## Start locally

1) Start infra:

```bash
docker compose up -d
```

2) Start services in separate terminals:

```bash
cd collector-service && make run
cd worker-service && make run
```

3) Run end-to-end smoke test:

```bash
./test.sh
```

## Important env vars

Worker (`worker-service/.env`):
- `ARCHIVE_BATCH_SIZE=100`
- `ARCHIVE_FLUSH_INTERVAL_SEC=5`
- `ARCHIVE_PREFIX_MODE=status`
- `FRAUD_IP_WINDOW_SEC=300`
- `FRAUD_IP_THRESHOLD=100`

## Check MinIO archived `.ndjson.gz` files

List archived objects by status prefix:

```bash
docker exec minio sh -c "mc alias set local http://localhost:9000 minioadmin minioadmin >/dev/null && mc ls --recursive local/event-archive/raw"
docker exec minio sh -c "mc ls --recursive local/event-archive/duplicate"
docker exec minio sh -c "mc ls --recursive local/event-archive/fraud"
docker exec minio sh -c "mc ls --recursive local/event-archive/accepted"
```

Copy one gzip file to local machine and inspect NDJSON lines:

```bash
# Example: copy first raw file
OBJ=$(docker exec minio sh -c "mc find local/event-archive/raw --name '*.ndjson.gz' | head -n 1")
docker exec minio sh -c "mc cp ${OBJ} /tmp/sample.ndjson.gz"
docker cp minio:/tmp/sample.ndjson.gz /tmp/sample.ndjson.gz

# Inspect
gzip -dc /tmp/sample.ndjson.gz | head
```

## Notes on test.sh

- By default Redis verification is strict (`STRICT_REDIS_CHECK=true`)
- Test fails if Redis counter reset/read is not possible or mismatched
- Override only if needed:

```bash
STRICT_REDIS_CHECK=false ./test.sh
```

## Unit tests

Run unit tests per service:

```bash
cd collector-service && go test ./...
cd query-service && go test ./...
cd worker-service && go test ./...
```

Current unit test focus:
- collector: client IP extraction header priority
- query: service validation and repository error wrapping
- worker: event validation and hour-bucket helpers

## CI and branch protection

GitHub Actions workflows are split by concern:
- `.github/workflows/ci.yml` for Go unit tests and Terraform validation
- `.github/workflows/deploy.yml` for image build/push and Kubernetes deploy
- `.github/workflows/infra.yml` for Terraform plan/apply pipeline

Recommended branch protection for `main`:
- require pull requests before merging
- require status checks to pass (`CI / Unit Tests`, `CI / Terraform Validate`)
- require branch to be up to date before merging
- restrict force pushes and branch deletion
