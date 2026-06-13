output "resource_group_name" {
  value       = azurerm_resource_group.this.name
  description = "Resource group that owns the vault."
}

output "vault_name" {
  value       = azurerm_key_vault.this.name
  description = "Key Vault name."
}

output "vault_id" {
  value       = azurerm_key_vault.this.id
  description = "Full ARM resource ID."
}

output "vault_uri" {
  value       = azurerm_key_vault.this.vault_uri
  description = "Vault URI (https://<name>.vault.azure.net/)."
}
