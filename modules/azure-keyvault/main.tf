locals {
  base_tags = merge(var.tags, {
    Environment = var.environment
    ManagedBy   = "opord"
  })
  # Vault name: 3-24 chars, must start with letter, alphanumeric + hyphens.
  # Strip invalid chars + cap to 20 + add `-kv` suffix.
  raw_name   = lower(replace(var.name_prefix, "/[^a-z0-9-]/", ""))
  trimmed    = substr(local.raw_name, 0, 20)
  vault_name = "${local.trimmed}-kv"
}

data "azurerm_client_config" "current" {}

resource "azurerm_resource_group" "this" {
  name     = "${var.name_prefix}-rg"
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_key_vault" "this" {
  name                       = local.vault_name
  resource_group_name        = azurerm_resource_group.this.name
  location                   = azurerm_resource_group.this.location
  tenant_id                  = data.azurerm_client_config.current.tenant_id
  sku_name                   = var.sku_name
  enable_rbac_authorization  = var.rbac_authorization_enabled
  purge_protection_enabled   = var.purge_protection_enabled
  soft_delete_retention_days = var.soft_delete_retention_days
  tags                       = local.base_tags
}
