# Dev AWS Foundation (Terraform)

This stack provisions:
- VPC (public/private subnets + NAT)
- EKS cluster + managed node group
- S3 archive bucket with lifecycle rules
- ElastiCache Redis
- RDS PostgreSQL

Cost-aware defaults for short-lived dev testing:
- EKS nodes run as `t3.small` (desired 1)
- NAT gateway is disabled by default (`enable_nat_gateway = false`)
- EKS nodes use public subnets in dev (`use_public_subnets_for_nodes = true`)
- RDS and Redis use micro classes

## Usage

1) Copy tfvars:

```bash
cp terraform.tfvars.example terraform.tfvars
```

2) Edit `terraform.tfvars` (especially `db_password`).

3) Initialize and apply:

```bash
terraform init
terraform plan
terraform apply
```

## Notes

- This is a dev baseline with small instance classes.
- EKS control plane itself is billed hourly by AWS even for short runs.
- Destroy the stack after testing to avoid ongoing cost:

```bash
terraform destroy
```
- RDS is private, non-public, `skip_final_snapshot = true`.
- S3 lifecycle rules are set for `raw/`, `duplicate/`, `fraud/`, `accepted/` prefixes.
- MSK is intentionally not included yet.
