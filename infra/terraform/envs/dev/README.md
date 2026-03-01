# Dev AWS Foundation (Terraform)

Terraform stack for running the event pipeline on AWS dev infrastructure.

## What this stack creates

- VPC with public/private subnets
- EKS cluster + managed node group
- EKS EBS CSI addon
- S3 archive bucket with lifecycle rules
- ElastiCache Redis
- RDS PostgreSQL
- IAM policies for EBS CSI and worker S3 archive writes

## Dev defaults

- small instance sizes for cost control
- NAT disabled by default
- nodes can run in public subnets for simpler dev bring-up
- public subnets map public IPs on launch (for dev nodegroup compatibility)

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:
- `aws_region`
- `db_password`

Apply:

```bash
terraform init
terraform plan -out plan.out
terraform apply plan.out
```

## Helpful outputs

```bash
terraform output
terraform output -raw eks_cluster_name
terraform output -raw s3_bucket_name
terraform output -raw redis_endpoint
terraform output -raw rds_endpoint
```

Export for convenience:

```bash
export EKS_CLUSTER_NAME=$(terraform output -raw eks_cluster_name)
export S3_BUCKET=$(terraform output -raw s3_bucket_name)
export REDIS_ENDPOINT=$(terraform output -raw redis_endpoint)
export RDS_ENDPOINT=$(terraform output -raw rds_endpoint)
```

## Cost note

This is still billable even when app traffic is low. Main contributors are EKS control plane, nodes, RDS, and ElastiCache.

## Destroy when done

```bash
terraform destroy
```

## Known design choices

- RDS is private and encrypted, with `skip_final_snapshot = true` for fast dev teardown.
- S3 lifecycle rules are configured for `raw/`, `duplicate/`, `fraud/`, and `accepted/` prefixes.
- Kafka is currently in-cluster Redpanda; managed Kafka can be added later.
