output "saml_provider_arn" {
  description = "ARN of the Azure AD SAML provider."
  value       = aws_iam_saml_provider.azuread.arn
}

output "role_arns" {
  description = "Map of role name to ARN for the federated roles."
  value       = { for k, r in aws_iam_role.this : k => r.arn }
}
