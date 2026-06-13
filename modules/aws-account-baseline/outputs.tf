output "budget_name" {
  description = "Name of the monthly cost budget."
  value       = aws_budgets_budget.monthly.name
}

output "config_recorder" {
  description = "AWS Config recorder name (empty when disabled)."
  value       = try(aws_config_configuration_recorder.this[0].name, "")
}

output "config_bucket" {
  description = "AWS Config delivery bucket (empty when disabled)."
  value       = try(aws_s3_bucket.config[0].id, "")
}

output "password_policy_applied" {
  value = true
}
