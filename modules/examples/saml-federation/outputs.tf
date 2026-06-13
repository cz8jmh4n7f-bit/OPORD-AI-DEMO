output "saml_provider_arn" {
  description = "ARN of the Azure AD SAML provider (paste into the Entra Role claim)."
  value       = aws_iam_saml_provider.azuread.arn
}

output "role_arns" {
  description = "Map of role name to ARN for the federated roles."
  value       = { for k, r in aws_iam_role.this : k => r.arn }
}

output "role_claim_values" {
  description = "Ready-to-paste Entra Role-claim values: <role_arn>,<provider_arn>."
  value       = { for k, r in aws_iam_role.this : k => "${r.arn},${aws_iam_saml_provider.azuread.arn}" }
}
