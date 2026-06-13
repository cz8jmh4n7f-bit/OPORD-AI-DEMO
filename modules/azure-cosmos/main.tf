locals {
  base_tags = merge(var.tags, {
    Environment = var.environment
    ManagedBy   = "opord"
  })
  # Cosmos account names are globally unique. Keep the friendly prefix but add
  # a deterministic suffix to avoid collisions across OPORD workspaces.
  raw_name     = lower(replace(var.name_prefix, "/[^a-z0-9-]/", ""))
  short_name   = substr(trim(local.raw_name, "-"), 0, 31)
  suffix       = substr(md5(var.name_prefix), 0, 6)
  account_name = "${local.short_name}-${local.suffix}-cos"
  db_name      = var.table_name
  serverless   = contains(["PAY_PER_REQUEST", "SERVERLESS"], var.billing_mode)
  autoscale    = var.billing_mode == "AUTOSCALE"
  provisioned  = var.billing_mode == "PROVISIONED"
}

resource "azurerm_resource_group" "this" {
  name     = "${var.name_prefix}-rg"
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_cosmosdb_account" "this" {
  name                = local.account_name
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  offer_type          = var.offer_type
  kind                = "GlobalDocumentDB"
  tags                = local.base_tags

  dynamic "capabilities" {
    for_each = local.serverless ? [1] : []
    content {
      name = "EnableServerless"
    }
  }

  consistency_policy {
    consistency_level = var.consistency_level
  }

  geo_location {
    location          = var.location
    failover_priority = 0
  }
}

resource "azurerm_cosmosdb_sql_database" "this" {
  name                = local.db_name
  resource_group_name = azurerm_resource_group.this.name
  account_name        = azurerm_cosmosdb_account.this.name
}

resource "azurerm_cosmosdb_sql_container" "this" {
  name                = var.table_name
  resource_group_name = azurerm_resource_group.this.name
  account_name        = azurerm_cosmosdb_account.this.name
  database_name       = azurerm_cosmosdb_sql_database.this.name
  partition_key_paths = [var.partition_key]
  # SERVERLESS/PAY_PER_REQUEST leave throughput unset. AUTOSCALE and
  # PROVISIONED are explicit because Azure bills them differently.
  throughput = local.provisioned ? var.throughput : null

  dynamic "autoscale_settings" {
    for_each = local.autoscale ? [1] : []
    content {
      max_throughput = var.max_throughput
    }
  }
}
