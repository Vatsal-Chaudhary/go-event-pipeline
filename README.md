# Go Event Pipeline

Simple event pipeline using Go, Redpanda (Kafka), Redis, PostgreSQL, and MinIO.

## Flow

`collector-service` -> Kafka `raw-events` -> `worker-service`

Worker does:
- archive `raw`
- dedupe by `event_id`
- fraud check by IP threshold (Redis-backed fraud-service)
- archive `duplicate` / `fraud` / `accepted`
- persist only accepted events to PostgreSQL

Archive files are batched NDJSON + gzip in MinIO:
- `raw/YYYY/MM/DD/HH/batch_*.ndjson.gz`
- `duplicate/YYYY/MM/DD/HH/batch_*.ndjson.gz`
- `fraud/YYYY/MM/DD/HH/batch_*.ndjson.gz`
- `accepted/YYYY/MM/DD/HH/batch_*.ndjson.gz`

## Start locally

1) Start infra and fraud-service:

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
- `FRAUD_MODE=http`
- `FRAUD_ENDPOINT=http://localhost:9090`
- `ARCHIVE_BATCH_SIZE=100`
- `ARCHIVE_FLUSH_INTERVAL_SEC=5`
- `ARCHIVE_PREFIX_MODE=status`

Fraud service (`docker-compose.yaml` env):
- `REDIS_ADDR=redis:6379`
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
