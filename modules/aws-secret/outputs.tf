output "secret_arn" {
  description = "Secret ARN (apps reference this in IAM policies + SDK)."
  value       = aws_secretsmanager_secret.this.arn
}

output "secret_id" {
  description = "Secret friendly id (same as name for new secrets)."
  value       = aws_secretsmanager_secret.this.id
}

output "secret_name" {
  description = "Secret name as supplied."
  value       = aws_secretsmanager_secret.this.name
}
