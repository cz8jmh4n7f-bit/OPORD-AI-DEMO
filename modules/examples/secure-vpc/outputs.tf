output "vpc_id" {
  description = "ID of the created VPC."
  value       = aws_vpc.this.id
}

output "vpc_cidr" {
  description = "CIDR of the VPC."
  value       = aws_vpc.this.cidr_block
}

output "subnet_ids" {
  description = "IDs of the per-AZ /24 subnets."
  value       = aws_subnet.this[*].id
}

output "default_sg_id" {
  description = "The locked-down default security group."
  value       = aws_default_security_group.this.id
}

output "flow_log_id" {
  description = "VPC Flow Log ID."
  value       = aws_flow_log.this.id
}
