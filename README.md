# Go Event Pipeline

Event-driven analytics pipeline in Go with local and AWS deployment paths.

Core runtime flow:

`collector-service` -> Kafka topic `raw-events` -> `worker-service` -> `query-service`

Worker flow per event:
- archive `raw`
- dedupe by `event_id` in Redis
- fraud score by IP window in Redis
- classify `duplicate` / `fraud` / `accepted`
- archive final status
- persist only accepted events to Postgres and upsert campaign aggregates

## Architecture

Services:
- `collector-service`: ingest API (`POST /event`, `POST /events/batch`)
- `worker-service`: async processing, dedupe, fraud scoring, archive, persistence
- `query-service`: read API (`GET /campaigns`, `GET /campaigns/:id`)

Data and infra:
- Kafka: Redpanda
- Redis: dedupe and fraud counters
- Postgres: `raw_events`, `campaign_stats`
- Archive: MinIO locally, S3 on AWS

## Repo layout

- `collector-service/`
- `worker-service/`
- `query-service/`
- `k8s/` Kubernetes base and overlays
- `infra/terraform/` AWS infrastructure
- `test.sh` full local/k8s integration test
- `test_ingest.sh` focused ingest test
- `test_retrieval.sh` focused query test
- `test_aws.sh` AWS-aware end-to-end test

## Quick start (local)

1. Start dependencies:

```bash
docker compose up -d
```

2. Start services in separate terminals:

```bash
cd collector-service && make run
cd worker-service && make run
cd query-service && make run
```

3. Run end-to-end test:

```bash
./test.sh
```

## Quick start (AWS)

Use these docs for full setup:
- `infra/terraform/envs/dev/README.md`
- `k8s/README.md`

Fast test path after deploy (when ingress is not exposed yet):

```bash
kubectl -n event-pipeline port-forward svc/collector-service 3000:80
kubectl -n event-pipeline port-forward svc/query-service 3002:80
```

Then run:

```bash
./test_ingest.sh --collector-url http://localhost:3000
./test_retrieval.sh --query-url http://localhost:3002
./test_aws.sh --collector-url http://localhost:3000 --query-url http://localhost:3002 --strict-s3-check true
```

## CI/CD

- `.github/workflows/ci.yml`: unit tests + Terraform validate
- `.github/workflows/infra.yml`: Terraform plan/apply/destroy
- `.github/workflows/deploy.yml`: build/push images + deploy overlay to EKS

## Runtime proof

Collected runtime screenshots are stored in `docs/assets/screenshots/`.

![Kubernetes Pods](docs/assets/screenshots/01-k8s-pods.png)
![AWS Test Pass](docs/assets/screenshots/02-aws-test-pass.png)
![Query Campaign Response](docs/assets/screenshots/03-query-campaign.png)
![S3 Archive Prefix Counts](docs/assets/screenshots/04-s3-prefix-counts.png)
![K9s Runtime View](docs/assets/screenshots/05-k9s-runtime.png)

## Notes

- This project is tuned as a portfolio-grade system with practical reliability checks.
- Typical production next steps: DLQ + replay pipeline, stronger observability/tracing, and full IRSA least-privilege IAM.
