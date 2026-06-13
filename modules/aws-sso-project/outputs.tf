output "group_id" {
  description = "Identity Center group ID for the project."
  value       = aws_identitystore_group.project.group_id
}

output "group_name" {
  description = "Identity Center group display name."
  value       = aws_identitystore_group.project.display_name
}

output "permission_set_arn" {
  description = "ARN of the permission set bound to the account (created or referenced)."
  value       = local.permission_set_arn
}

output "account_id" {
  description = "Target AWS account the group was granted access to."
  value       = var.account_id
}

output "member_count" {
  description = "Number of users assigned to the project group."
  value       = length(var.user_names)
}

output "instance_arn" {
  description = "Identity Center instance ARN used."
  value       = local.instance_arn
}
