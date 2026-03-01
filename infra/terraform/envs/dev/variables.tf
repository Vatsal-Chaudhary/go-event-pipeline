variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "ap-south-1"
}

variable "project_name" {
  description = "Project name prefix"
  type        = string
  default     = "go-event-pipeline"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "dev"
}

variable "vpc_cidr" {
  description = "VPC CIDR block"
  type        = string
  default     = "10.30.0.0/16"
}

variable "eks_cluster_version" {
  description = "EKS Kubernetes version"
  type        = string
  default     = "1.30"
}

variable "eks_node_instance_types" {
  description = "EKS managed node group instance types"
  type        = list(string)
  default     = ["t3.small"]
}

variable "eks_node_desired_size" {
  description = "EKS managed node group desired size"
  type        = number
  default     = 1
}

variable "eks_node_min_size" {
  description = "EKS managed node group min size"
  type        = number
  default     = 1
}

variable "eks_node_max_size" {
  description = "EKS managed node group max size"
  type        = number
  default     = 2
}

variable "enable_nat_gateway" {
  description = "Enable NAT gateway (adds hourly cost)"
  type        = bool
  default     = false
}

variable "use_public_subnets_for_nodes" {
  description = "Run EKS nodes in public subnets for low-cost dev"
  type        = bool
  default     = true
}

variable "db_name" {
  description = "Postgres DB name"
  type        = string
  default     = "analytics"
}

variable "db_username" {
  description = "Postgres master username"
  type        = string
  default     = "postgres"
}

variable "db_password" {
  description = "Postgres master password"
  type        = string
  sensitive   = true
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t4g.micro"
}

variable "db_allocated_storage" {
  description = "RDS allocated storage (GiB)"
  type        = number
  default     = 20
}

variable "redis_node_type" {
  description = "ElastiCache node type"
  type        = string
  default     = "cache.t4g.micro"
}
