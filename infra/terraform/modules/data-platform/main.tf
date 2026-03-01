locals {
  bucket_name = "${var.name_prefix}-event-archive"
}

resource "aws_s3_bucket" "archive" {
  bucket        = local.bucket_name
  force_destroy = var.s3_force_destroy
}

resource "aws_s3_bucket_versioning" "archive" {
  bucket = aws_s3_bucket.archive.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "archive" {
  bucket = aws_s3_bucket.archive.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "archive" {
  bucket                  = aws_s3_bucket.archive.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_lifecycle_configuration" "archive" {
  bucket = aws_s3_bucket.archive.id

  rule {
    id     = "raw-expiry"
    status = "Enabled"
    filter {
      prefix = "raw/"
    }
    expiration {
      days = 14
    }
  }

  rule {
    id     = "duplicate-expiry"
    status = "Enabled"
    filter {
      prefix = "duplicate/"
    }
    expiration {
      days = 1
    }
  }

  rule {
    id     = "fraud-expiry"
    status = "Enabled"
    filter {
      prefix = "fraud/"
    }
    expiration {
      days = 30
    }
  }

  rule {
    id     = "accepted-expiry"
    status = "Enabled"
    filter {
      prefix = "accepted/"
    }
    expiration {
      days = 90
    }
  }
}

resource "aws_security_group" "redis" {
  name        = "${var.name_prefix}-redis-sg"
  description = "Redis access from EKS worker nodes"
  vpc_id      = var.vpc_id
}

resource "aws_security_group_rule" "redis_ingress" {
  type                     = "ingress"
  from_port                = 6379
  to_port                  = 6379
  protocol                 = "tcp"
  security_group_id        = aws_security_group.redis.id
  source_security_group_id = var.eks_node_security_group_id
}

resource "aws_security_group_rule" "redis_egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  security_group_id = aws_security_group.redis.id
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_elasticache_subnet_group" "redis" {
  name       = "${var.name_prefix}-redis-subnet-group"
  subnet_ids = var.private_subnet_ids
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id       = "${var.name_prefix}-redis"
  description                = "Redis for event pipeline"
  engine                     = "redis"
  engine_version             = "7.1"
  node_type                  = var.redis_node_type
  num_cache_clusters         = 1
  automatic_failover_enabled = false
  subnet_group_name          = aws_elasticache_subnet_group.redis.name
  security_group_ids         = [aws_security_group.redis.id]
  port                       = 6379
}

resource "aws_security_group" "rds" {
  name        = "${var.name_prefix}-rds-sg"
  description = "Postgres access from EKS worker nodes"
  vpc_id      = var.vpc_id
}

resource "aws_security_group_rule" "rds_ingress" {
  type                     = "ingress"
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  security_group_id        = aws_security_group.rds.id
  source_security_group_id = var.eks_node_security_group_id
}

resource "aws_security_group_rule" "rds_egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  security_group_id = aws_security_group.rds.id
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_db_subnet_group" "postgres" {
  name       = "${var.name_prefix}-postgres-subnet-group"
  subnet_ids = var.private_subnet_ids
}

resource "aws_db_instance" "postgres" {
  identifier              = "${var.name_prefix}-postgres"
  allocated_storage       = var.db_allocated_storage
  engine                  = "postgres"
  instance_class          = var.db_instance_class
  db_name                 = var.db_name
  username                = var.db_username
  password                = var.db_password
  db_subnet_group_name    = aws_db_subnet_group.postgres.name
  vpc_security_group_ids  = [aws_security_group.rds.id]
  publicly_accessible     = false
  skip_final_snapshot     = true
  backup_retention_period = 1
  storage_encrypted       = true
}
