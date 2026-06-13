output "log_analytics_workspace_id" {
  value       = azurerm_log_analytics_workspace.this.id
  description = "LAW resource ID. Future workloads point Diagnostic Settings here."
}

output "log_analytics_workspace_name" {
  value       = azurerm_log_analytics_workspace.this.name
  description = "LAW name."
}

output "archive_storage_account_id" {
  value       = azurerm_storage_account.archive.id
  description = "Archive SA ID. L4 secure-vnet uses this as flow_logs_storage_account_id."
}

output "archive_storage_account_name" {
  value       = azurerm_storage_account.archive.name
  description = "Archive SA name."
}
