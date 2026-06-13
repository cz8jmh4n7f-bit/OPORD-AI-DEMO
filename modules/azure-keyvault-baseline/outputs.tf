output "vault_name" {
  value       = azurerm_key_vault.this.name
  description = "Key Vault name."
}

output "vault_id" {
  value       = azurerm_key_vault.this.id
  description = "Full ARM resource ID (used as scope for further RBAC)."
}

output "vault_uri" {
  value       = azurerm_key_vault.this.vault_uri
  description = "Vault URI (https://<name>.vault.azure.net/)."
}

output "cmk_id" {
  value       = var.create_cmk ? azurerm_key_vault_key.cmk[0].id : ""
  description = "CMK resource ID - consumed by L5 Storage Account encryption_scope. Empty when create_cmk=false."
}

output "cmk_versionless_id" {
  value       = var.create_cmk ? azurerm_key_vault_key.cmk[0].versionless_id : ""
  description = "CMK versionless ID - preferred for auto-rotation (Azure picks the current version)."
}
