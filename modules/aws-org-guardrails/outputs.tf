output "scp_policy_id" {
  description = "ID of the Service Control Policy."
  value       = aws_organizations_policy.scp.id
}

output "scp_policy_arn" {
  description = "ARN of the Service Control Policy."
  value       = aws_organizations_policy.scp.arn
}

output "tag_policy_id" {
  description = "ID of the tag policy (empty when disabled)."
  value       = try(aws_organizations_policy.tags[0].id, "")
}

output "attached_targets" {
  description = "Targets the guardrails were attached to."
  value       = var.target_ids
}
