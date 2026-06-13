output "resource_group_name" {
  value       = azurerm_resource_group.this.name
  description = "Resource group that owns the server."
}

output "server_name" {
  value       = azurerm_postgresql_flexible_server.this.name
  description = "Server name."
}

output "server_id" {
  value       = azurerm_postgresql_flexible_server.this.id
  description = "Full ARM resource ID."
}

output "endpoint" {
  value       = azurerm_postgresql_flexible_server.this.fqdn
  description = "Server FQDN (use as the Postgres host)."
}

output "port" {
  value       = 5432
  description = "Postgres TCP port (Azure default)."
}

output "admin_username" {
  value       = azurerm_postgresql_flexible_server.this.administrator_login
  description = "Admin user."
}

output "admin_password" {
  value       = random_password.admin.result
  description = "Admin password (in tofu state - treat the workspace as a secret)."
  sensitive   = true
}

output "database_name" {
  value       = var.database_name != "" ? var.database_name : null
  description = "Initial database name (or null when none)."
}
