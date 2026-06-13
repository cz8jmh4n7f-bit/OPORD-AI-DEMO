output "account_id" {
  description = "The new member account's 12-digit ID."
  value       = aws_organizations_account.this.id
}

output "account_arn" {
  description = "ARN of the member account."
  value       = aws_organizations_account.this.arn
}

output "access_role_arn" {
  description = "ARN of the bootstrap role the later layers assume into."
  value       = "arn:aws:iam::${aws_organizations_account.this.id}:role/${var.access_role_name}"
}

output "account_name" {
  value = aws_organizations_account.this.name
}
