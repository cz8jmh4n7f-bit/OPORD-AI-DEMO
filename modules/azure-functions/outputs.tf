output "resource_group_name" {
  value       = azurerm_resource_group.this.name
  description = "Resource group that owns the function app."
}

output "function_name" {
  value       = azurerm_linux_function_app.this.name
  description = "Function App name."
}

output "function_id" {
  value       = azurerm_linux_function_app.this.id
  description = "Full ARM resource ID."
}

output "default_hostname" {
  value       = azurerm_linux_function_app.this.default_hostname
  description = "Default hostname (https://<name>.azurewebsites.net)."
}

output "storage_account_name" {
  value       = azurerm_storage_account.this.name
  description = "Backing storage account name."
}
