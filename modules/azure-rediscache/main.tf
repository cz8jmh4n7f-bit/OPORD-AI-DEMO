locals {
  base_tags = merge(var.tags, {
    Environment = var.environment
    ManagedBy   = "opord"
  })
  # Redis cache names are a global DNS label: 1-63 chars, alphanumeric + hyphens.
  raw_name   = lower(replace(var.name_prefix, "/[^a-z0-9-]/", ""))
  cache_name = substr("${local.raw_name}-redis", 0, 63)
}

resource "azurerm_resource_group" "this" {
  name     = "${var.name_prefix}-rg"
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_redis_cache" "this" {
  name                 = local.cache_name
  location             = azurerm_resource_group.this.location
  resource_group_name  = azurerm_resource_group.this.name
  capacity             = var.capacity
  family               = var.family
  sku_name             = var.sku_name
  redis_version        = var.redis_version
  non_ssl_port_enabled = false # TLS-only
  minimum_tls_version  = "1.2"
  tags                 = local.base_tags
}
