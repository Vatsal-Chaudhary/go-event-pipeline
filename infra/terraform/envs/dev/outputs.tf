output "vpc_id" {
  value = module.vpc.vpc_id
}

output "private_subnet_ids" {
  value = module.vpc.private_subnets
}

output "eks_cluster_name" {
  value = module.eks.cluster_name
}

output "eks_cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

output "eks_cluster_ca_data" {
  value = module.eks.cluster_certificate_authority_data
}

output "s3_bucket_name" {
  value = module.data_platform.s3_bucket_name
}

output "redis_endpoint" {
  value = module.data_platform.redis_endpoint
}

output "redis_port" {
  value = module.data_platform.redis_port
}

output "rds_endpoint" {
  value = module.data_platform.rds_endpoint
}

output "rds_port" {
  value = module.data_platform.rds_port
}

output "rds_db_name" {
  value = module.data_platform.rds_db_name
}

output "rds_username" {
  value = module.data_platform.rds_username
}
