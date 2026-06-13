output "vpc_id" {
  description = "ID of the created VPC."
  value       = aws_vpc.this.id
}

output "vpc_cidr" {
  description = "VPC CIDR block."
  value       = aws_vpc.this.cidr_block
}

output "subnet_ids" {
  description = "IDs of the per-AZ subnets."
  value       = aws_subnet.this[*].id
}

output "availability_zones" {
  description = "AZs the subnets span."
  value       = local.azs
}

output "flow_log_group" {
  description = "CloudWatch log group for VPC Flow Logs."
  value       = aws_cloudwatch_log_group.flow.name
}
