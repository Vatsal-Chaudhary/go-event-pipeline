# Kubernetes Manifests

Kubernetes manifests are split to keep app definitions reusable across local and AWS.

## Layout

```
k8s/
  base/                         shared manifests (namespace, infra, apps, ingress)
    config/                     dev-default ConfigMap and Secret
    apps/                       collector, worker, query Deployments + Services
    infra/                      in-cluster Postgres, Redis, Redpanda, MinIO
    ingress/                    Ingress rules (nginx, *.local hosts)
  overlays/
    local/                      passthrough overlay (uses base as-is)
    aws/
      aws-configmap.yaml        template ConfigMap with AWS values (edit before apply)
      aws-secrets.yaml          template Secret with AWS values (edit before apply)
      patches/                  strategic-merge patches for AWS runtime
```

## Local apply

```bash
kubectl apply -k k8s/overlays/local
```

This deploys everything in-cluster: Redpanda, Redis, Postgres, MinIO, and the three app services with dev defaults.

## AWS deploy flow

The AWS overlay deletes the base ConfigMap and Secret (dev defaults) from the rendered output. Deployments still reference `event-pipeline-config` and `event-pipeline-secrets` by name. If those don't exist in the namespace when pods start, you get `CreateContainerConfigError`.

The correct deploy order is strict:

### Step 1: Provision infrastructure

```bash
cd infra/terraform/envs/dev
cp terraform.tfvars.example terraform.tfvars
# edit terraform.tfvars with real values (especially db_password)
terraform init
terraform plan -out plan.out
terraform apply plan.out
```

Export outputs for use in the next steps:

```bash
export S3_BUCKET=$(terraform output -raw s3_bucket_name)
export REDIS_ENDPOINT=$(terraform output -raw redis_endpoint)
export RDS_ENDPOINT=$(terraform output -raw rds_endpoint)
export EKS_CLUSTER_NAME=$(terraform output -raw eks_cluster_name)
```

### Step 2: Connect to EKS

```bash
aws eks update-kubeconfig --name "$EKS_CLUSTER_NAME" --region ap-south-1
```

### Step 3: Create namespace

```bash
kubectl apply -f k8s/base/namespace.yaml
```

### Step 4: Create ConfigMap and Secret with real AWS values

Copy the templates from the overlay directory and fill in real values:

```bash
cp k8s/overlays/aws/aws-configmap.yaml /tmp/aws-configmap.yaml
cp k8s/overlays/aws/aws-secrets.yaml /tmp/aws-secrets.yaml
```

Edit `/tmp/aws-configmap.yaml`:
- `REDIS_ADDR` -> `$REDIS_ENDPOINT:6379`
- `S3_BUCKET` -> `$S3_BUCKET`

Edit `/tmp/aws-secrets.yaml`:
- `DB_URL` -> `postgres://postgres:<password>@$RDS_ENDPOINT:5432/analytics?sslmode=require`

Apply them:

```bash
kubectl -n event-pipeline apply -f /tmp/aws-configmap.yaml
kubectl -n event-pipeline apply -f /tmp/aws-secrets.yaml
```

### Step 5: Apply the overlay

```bash
kubectl apply -k k8s/overlays/aws
```

### Step 6: Verify

```bash
kubectl -n event-pipeline get pods
kubectl -n event-pipeline rollout status deploy/collector-service
kubectl -n event-pipeline rollout status deploy/worker-service
kubectl -n event-pipeline rollout status deploy/query-service
```

## AWS overlay behavior

- replaces base ConfigMap/Secret with externally-managed versions (must exist before apply)
- sets `WORKER_ARCHIVE_BACKEND=s3` in the template ConfigMap
- removes in-cluster MinIO resources (apps use S3)
- removes in-cluster Postgres resources (apps use RDS via `DB_URL`)
- removes in-cluster Redis resources (apps use ElastiCache via `REDIS_ADDR`)
- keeps in-cluster Redpanda (Kafka); replace with MSK later if needed
- sets collector/worker/query replicas to 1 for small dev clusters
- removes worker HPA to avoid noisy autoscaling dependencies
- replaces `*.local` nginx Ingress with ALB Ingress (no host constraint)

## Access for testing

If ALB ingress controller is not installed, use port-forward:

```bash
kubectl -n event-pipeline port-forward svc/collector-service 3000:80
kubectl -n event-pipeline port-forward svc/query-service 3002:80
```

Then run project tests from repo root:

```bash
./test_ingest.sh --collector-url http://localhost:3000
./test_retrieval.sh --query-url http://localhost:3002
./test_aws.sh --collector-url http://localhost:3000 --query-url http://localhost:3002 --strict-s3-check true
```

## Notes

- Worker DB migrations run at startup; worker image must include `internal/db/migrations`.
- Base secrets are dev defaults only; for non-local environments, provide secrets at deploy time.
- The `aws-configmap.yaml` and `aws-secrets.yaml` in the overlay directory are templates with `REPLACE_ME` placeholders. Never commit real credentials to these files.
