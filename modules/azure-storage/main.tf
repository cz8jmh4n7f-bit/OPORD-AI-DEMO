locals {
  base_tags = merge(var.tags, {
    Environment = var.environment
    ManagedBy   = "opord"
  })
  # Storage account names are globally unique. Add a deterministic hash so two
  # OPORD projects with the same friendly name do not collide.
  raw_name     = lower(replace(var.name_prefix, "/[^a-z0-9]/", ""))
  short_name   = substr(local.raw_name, 0, 16)
  suffix       = substr(md5(var.name_prefix), 0, 6)
  account_name = "${local.short_name}${local.suffix}sa"
  rg_name      = "${var.name_prefix}-rg"
}

resource "azurerm_resource_group" "this" {
  name     = local.rg_name
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_storage_account" "this" {
  name                            = local.account_name
  resource_group_name             = azurerm_resource_group.this.name
  location                        = azurerm_resource_group.this.location
  account_tier                    = var.account_tier
  account_replication_type        = var.replication_type
  allow_nested_items_to_be_public = var.allow_blob_public_access
  min_tls_version                 = "TLS1_2"
  tags                            = local.base_tags

  blob_properties {
    versioning_enabled = var.versioning
  }
}

resource "azurerm_storage_container" "this" {
  count                 = length(var.containers)
  name                  = var.containers[count.index]
  storage_account_id    = azurerm_storage_account.this.id
  container_access_type = "private"
}
