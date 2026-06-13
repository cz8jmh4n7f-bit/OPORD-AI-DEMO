output "resource_group_name" {
  value       = azurerm_resource_group.this.name
  description = "Resource group that owns the namespace."
}

output "namespace_name" {
  value       = azurerm_servicebus_namespace.this.name
  description = "Service Bus namespace name."
}

output "namespace_id" {
  value       = azurerm_servicebus_namespace.this.id
  description = "Full ARM resource ID."
}

output "endpoint" {
  value       = azurerm_servicebus_namespace.this.endpoint
  description = "Namespace endpoint URL."
}

output "queue_names" {
  value       = [for q in azurerm_servicebus_queue.this : q.name]
  description = "Created queue names."
}

# Default SAS connection string (RootManageSharedAccessKey policy is auto-created).
output "default_connection_string" {
  value       = azurerm_servicebus_namespace.this.default_primary_connection_string
  description = "Root SAS connection string. SENSITIVE."
  sensitive   = true
}
