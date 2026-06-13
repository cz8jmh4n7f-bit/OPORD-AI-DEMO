output "cache_name" {
  value = azurerm_redis_cache.this.name
}

output "cache_id" {
  value = azurerm_redis_cache.this.id
}

output "hostname" {
  value = azurerm_redis_cache.this.hostname
}

output "ssl_port" {
  value = azurerm_redis_cache.this.ssl_port
}

output "primary_endpoint" {
  description = "host:ssl_port for TLS Redis clients."
  value       = "${azurerm_redis_cache.this.hostname}:${azurerm_redis_cache.this.ssl_port}"
}

# NOTE: the primary/secondary access keys are intentionally NOT output -
# OPORD never persists credentials. Read them from the Azure portal / CLI.
