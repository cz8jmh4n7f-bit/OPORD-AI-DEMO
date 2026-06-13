locals {
  base_tags = merge(var.tags, {
    Project   = var.name_prefix
    CsaId     = var.csa_id
    ManagedBy = "opord"
  })
  raw_name      = lower(replace("${var.name_prefix}${var.csa_id}", "/[^a-z0-9]/", ""))
  archive_name  = substr("${local.raw_name}logssa", 0, 24)
  law_name      = "${var.name_prefix}-${var.csa_id}-logs-law"
  diag_name     = "${var.name_prefix}-${var.csa_id}-activitylog"
}

# Log Analytics: receiver for Diagnostic Settings + Defender for Cloud +
# future Sentinel binding.
resource "azurerm_log_analytics_workspace" "this" {
  name                = local.law_name
  location            = var.location
  resource_group_name = var.logs_rg_name
  sku                 = "PerGB2018"
  retention_in_days   = var.law_retention_days
  tags                = local.base_tags
}

# Storage Account: long-term archive for Activity Log + Flow Logs.
# Cool tier (cheap), versioning + soft delete (safety), CMK opt-in.
resource "azurerm_storage_account" "archive" {
  name                            = local.archive_name
  resource_group_name             = var.logs_rg_name
  location                        = var.location
  account_tier                    = "Standard"
  account_replication_type        = "GRS"
  account_kind                    = "StorageV2"
  access_tier                     = "Cool"
  min_tls_version                 = "TLS1_2"
  allow_nested_items_to_be_public = false
  shared_access_key_enabled       = true
  tags                            = local.base_tags

  identity {
    type = "SystemAssigned"
  }

  blob_properties {
    versioning_enabled = true
    delete_retention_policy {
      days = var.archive_storage_retention_days
    }
    container_delete_retention_policy {
      days = var.archive_storage_retention_days
    }
  }
}

# CMK encryption opt-in. The SA's system-assigned identity needs Key Vault
# Crypto Service Encryption User on the KV scope; the orchestrator grants
# that out-of-band (during the L5 step) - direct azurerm_role_assignment
# would need the KV resource ID, which we don't take as a var here to keep
# the module loose-coupled.
resource "azurerm_storage_account_customer_managed_key" "cmk" {
  count                     = var.cmk_versionless_id != "" ? 1 : 0
  storage_account_id        = azurerm_storage_account.archive.id
  key_vault_key_id          = var.cmk_versionless_id
}

# Activity Log to Log Analytics + Storage Account. Subscription-scope diag
# settings cover EVERY ARM event on this subscription, including admin role
# changes - the audit primitive.
resource "azurerm_monitor_diagnostic_setting" "activity_log" {
  name                       = local.diag_name
  target_resource_id         = var.subscription_resource_id
  log_analytics_workspace_id = azurerm_log_analytics_workspace.this.id
  storage_account_id         = azurerm_storage_account.archive.id

  # allLogs = every available log category for this target, without enumerating
  # them. Robust across subscription types/ages: a brand-new (create-mode)
  # subscription's diagnostic-categories API may not list every category at plan
  # time, which broke the enumerated form (plan errored on the unavailable
  # category while LAW+SA still planned to "2 to add" + exit 1).
  enabled_log {
    category_group = "allLogs"
  }
}
