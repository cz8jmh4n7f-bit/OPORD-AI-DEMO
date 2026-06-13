locals {
  base_tags = merge(var.tags, {
    Environment = var.environment
    ManagedBy   = "opord"
  })
  raw_name      = lower(replace(var.name_prefix, "/[^a-z0-9]/", ""))
  short_name    = substr(local.raw_name, 0, 14)
  suffix        = substr(md5(var.name_prefix), 0, 6)
  storage_name  = "${local.short_name}${local.suffix}fnsa"
  function_name = "${var.name_prefix}-fn"
  plan_name     = "${var.name_prefix}-plan"
}

resource "azurerm_resource_group" "this" {
  name     = "${var.name_prefix}-rg"
  location = var.location
  tags     = local.base_tags
}

# Functions require a backing Storage Account for blob/queue/table state.
resource "azurerm_storage_account" "this" {
  name                     = local.storage_name
  resource_group_name      = azurerm_resource_group.this.name
  location                 = azurerm_resource_group.this.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  min_tls_version          = "TLS1_2"
  tags                     = local.base_tags
}

# Y1 = Consumption (serverless, scales to zero, generous free tier).
resource "azurerm_service_plan" "this" {
  name                = local.plan_name
  resource_group_name = azurerm_resource_group.this.name
  location            = azurerm_resource_group.this.location
  os_type             = "Linux"
  sku_name            = "Y1"
  tags                = local.base_tags
}

resource "azurerm_linux_function_app" "this" {
  name                       = local.function_name
  resource_group_name        = azurerm_resource_group.this.name
  location                   = azurerm_resource_group.this.location
  service_plan_id            = azurerm_service_plan.this.id
  storage_account_name       = azurerm_storage_account.this.name
  storage_account_access_key = azurerm_storage_account.this.primary_access_key

  site_config {
    application_stack {
      python_version          = var.runtime == "python" ? var.runtime_version : null
      node_version            = var.runtime == "node" ? var.runtime_version : null
      java_version            = var.runtime == "java" ? var.runtime_version : null
      dotnet_version          = var.runtime == "dotnet" ? var.runtime_version : null
      powershell_core_version = var.runtime == "powershell" ? var.runtime_version : null
    }
  }

  app_settings = var.env_vars

  tags = local.base_tags
}
