provider "aws" {
  region = var.aws_region
}

data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  name_prefix = "${var.project_name}-${var.environment}"
  azs         = slice(data.aws_availability_zones.available.names, 0, 2)
  node_subnet_ids = var.use_public_subnets_for_nodes ? module.vpc.public_subnets : module.vpc.private_subnets
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${local.name_prefix}-vpc"
  cidr = var.vpc_cidr

  azs             = local.azs
  private_subnets = [for idx, _ in local.azs : cidrsubnet(var.vpc_cidr, 4, idx)]
  public_subnets  = [for idx, _ in local.azs : cidrsubnet(var.vpc_cidr, 4, idx + 8)]

  enable_nat_gateway = var.enable_nat_gateway
  single_nat_gateway = true
  map_public_ip_on_launch = true

  tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = "${local.name_prefix}-eks"
  cluster_version = var.eks_cluster_version

  cluster_endpoint_public_access = true

  vpc_id     = module.vpc.vpc_id
  subnet_ids = concat(module.vpc.private_subnets, module.vpc.public_subnets)

  enable_cluster_creator_admin_permissions = true

  eks_managed_node_groups = {
    default = {
      instance_types = var.eks_node_instance_types
      desired_size   = var.eks_node_desired_size
      min_size       = var.eks_node_min_size
      max_size       = var.eks_node_max_size
      subnet_ids     = local.node_subnet_ids
    }
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

module "data_platform" {
  source = "../../modules/data-platform"

  name_prefix                = local.name_prefix
  vpc_id                     = module.vpc.vpc_id
  private_subnet_ids         = module.vpc.private_subnets
  eks_node_security_group_id = module.eks.node_security_group_id

  db_name              = var.db_name
  db_username          = var.db_username
  db_password          = var.db_password
  db_instance_class    = var.db_instance_class
  db_allocated_storage = var.db_allocated_storage

  redis_node_type = var.redis_node_type
}
