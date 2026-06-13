output "resource_group_name" {
  value       = azurerm_resource_group.this.name
  description = "Resource group that owns the Cosmos account."
}

output "account_name" {
  value       = azurerm_cosmosdb_account.this.name
  description = "Cosmos DB account name."
}

output "account_id" {
  value       = azurerm_cosmosdb_account.this.id
  description = "Full ARM resource ID."
}

output "endpoint" {
  value       = azurerm_cosmosdb_account.this.endpoint
  description = "Cosmos DB endpoint URL (https://<account>.documents.azure.com:443/)."
}

output "database_name" {
  value       = azurerm_cosmosdb_sql_database.this.name
  description = "SQL database name."
}

output "container_name" {
  value       = azurerm_cosmosdb_sql_container.this.name
  description = "Container name (the DynamoDB-equivalent of `table_name`)."
}
