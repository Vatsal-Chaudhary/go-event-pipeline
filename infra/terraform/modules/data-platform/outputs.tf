output "s3_bucket_name" {
  value = aws_s3_bucket.archive.bucket
}

output "redis_endpoint" {
  value = aws_elasticache_replication_group.redis.primary_endpoint_address
}

output "redis_port" {
  value = aws_elasticache_replication_group.redis.port
}

output "rds_endpoint" {
  value = aws_db_instance.postgres.address
}

output "rds_port" {
  value = aws_db_instance.postgres.port
}

output "rds_db_name" {
  value = aws_db_instance.postgres.db_name
}

output "rds_username" {
  value = aws_db_instance.postgres.username
}
