locals {
  base_tags = merge(var.tags, {
    Environment = var.environment
    ManagedBy   = "opord"
  })
  server_name = "${var.name_prefix}-pg"
}

resource "azurerm_resource_group" "this" {
  name     = "${var.name_prefix}-rg"
  location = var.location
  tags     = local.base_tags
}

# Generate a strong admin password (24 chars, mixed case + digits + symbols).
# Lives in tofu state - treat the workspace state row as a secret.
resource "random_password" "admin" {
  length      = 24
  special     = true
  min_lower   = 2
  min_upper   = 2
  min_numeric = 2
  min_special = 2
  # Azure rejects $#&|' and a few others in passwords; keep our set safe.
  override_special = "!@*-_=+"
}

resource "azurerm_postgresql_flexible_server" "this" {
  name                          = local.server_name
  resource_group_name           = azurerm_resource_group.this.name
  location                      = azurerm_resource_group.this.location
  version                       = var.postgres_version
  sku_name                      = var.sku_name
  storage_mb                    = var.storage_mb
  administrator_login           = var.admin_username
  administrator_password        = random_password.admin.result
  public_network_access_enabled = var.allow_public_access
  zone                          = "1"
  tags                          = local.base_tags
}

resource "azurerm_postgresql_flexible_server_firewall_rule" "allow_all" {
  count            = var.allow_public_access ? 1 : 0
  name             = "allow-all-dev"
  server_id        = azurerm_postgresql_flexible_server.this.id
  start_ip_address = "0.0.0.0"
  end_ip_address   = "255.255.255.255"
}

resource "azurerm_postgresql_flexible_server_database" "this" {
  count     = var.database_name != "" ? 1 : 0
  name      = var.database_name
  server_id = azurerm_postgresql_flexible_server.this.id
  collation = "en_US.utf8"
  charset   = "UTF8"
}
