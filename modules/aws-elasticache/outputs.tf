output "primary_endpoint_address" {
  description = "Hostname clients connect to (writes + reads on single node)."
  value       = aws_elasticache_replication_group.this.primary_endpoint_address
}

output "reader_endpoint_address" {
  description = "Reader endpoint for read replicas (single-node clusters get the same as primary)."
  value       = aws_elasticache_replication_group.this.reader_endpoint_address
}

output "port" {
  description = "Redis port (always 6379)."
  value       = aws_elasticache_replication_group.this.port
}

output "replication_group_id" {
  description = "Replication group id."
  value       = aws_elasticache_replication_group.this.id
}
