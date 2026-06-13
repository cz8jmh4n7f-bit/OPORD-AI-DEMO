output "resource_group_name" {
  value       = azurerm_resource_group.this.name
  description = "Resource group that owns the storage account."
}

output "account_name" {
  value       = azurerm_storage_account.this.name
  description = "Storage account name (used in URLs / connection strings)."
}

output "account_id" {
  value       = azurerm_storage_account.this.id
  description = "Full ARM resource ID."
}

output "primary_blob_endpoint" {
  value       = azurerm_storage_account.this.primary_blob_endpoint
  description = "Primary blob endpoint URL."
}

output "container_names" {
  value       = [for c in azurerm_storage_container.this : c.name]
  description = "Created container names (empty when no containers requested)."
}
