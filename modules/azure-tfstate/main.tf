locals {
  base_tags = merge(var.tags, {
    Purpose   = "opord-tfstate"
    ManagedBy = "opord"
  })
  # SA names: 3-24 chars, lowercase alphanumeric only. Strip + truncate so the
  # final value fits even when name_prefix already contains hyphens.
  raw_name = lower(replace(var.name_prefix, "/[^a-z0-9]/", ""))
  sa_name  = substr("${local.raw_name}tfstate", 0, 24)
  rg_name  = "${var.name_prefix}-tfstate-rg"
}

resource "azurerm_resource_group" "this" {
  name     = local.rg_name
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_storage_account" "this" {
  name                            = local.sa_name
  resource_group_name             = azurerm_resource_group.this.name
  location                        = azurerm_resource_group.this.location
  account_tier                    = "Standard"
  account_replication_type        = "GRS"
  account_kind                    = "StorageV2"
  min_tls_version                 = "TLS1_2"
  allow_nested_items_to_be_public = false
  shared_access_key_enabled       = true
  tags                            = local.base_tags

  blob_properties {
    versioning_enabled  = true
    change_feed_enabled = true

    delete_retention_policy {
      days = var.soft_delete_retention_days
    }

    container_delete_retention_policy {
      days = var.soft_delete_retention_days
    }
  }
}

resource "azurerm_storage_container" "tfstate" {
  name                  = var.container_name
  storage_account_id    = azurerm_storage_account.this.id
  container_access_type = "private"
}
