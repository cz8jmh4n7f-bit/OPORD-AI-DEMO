output "arn" {
  description = "ARN of the ACM certificate."
  value       = aws_acm_certificate.this.arn
}

output "domain" {
  description = "Primary domain name of the certificate."
  value       = aws_acm_certificate.this.domain_name
}

output "status" {
  description = "Certificate status (ISSUED once validated, otherwise PENDING_VALIDATION)."
  value       = aws_acm_certificate.this.status
}
