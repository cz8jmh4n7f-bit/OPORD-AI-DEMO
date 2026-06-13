output "dns_name" {
  description = "DNS name of the load balancer (point a CNAME/alias at this)."
  value       = aws_lb.this.dns_name
}

output "arn" {
  description = "ARN of the load balancer."
  value       = aws_lb.this.arn
}

output "zone_id" {
  description = "Canonical hosted zone id of the ALB (for Route53 alias records)."
  value       = aws_lb.this.zone_id
}

output "target_group_arn" {
  description = "ARN of the target group traffic is forwarded to."
  value       = local.target_group_arn
}
