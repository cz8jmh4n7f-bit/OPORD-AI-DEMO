locals {
  base_tags = merge(var.tags, {
    Project   = var.name_prefix
    CsaId     = var.csa_id
    ManagedBy = "opord"
  })
  raw_name   = lower(replace("${var.name_prefix}-${var.csa_id}", "/[^a-z0-9-]/", ""))
  vault_name = substr("${local.raw_name}-kv", 0, 24)
  key_name   = "${var.csa_id}-cmk"
}

data "azurerm_client_config" "current" {}

resource "azurerm_key_vault" "this" {
  name                       = local.vault_name
  resource_group_name        = var.security_rg_name
  location                   = var.location
  tenant_id                  = data.azurerm_client_config.current.tenant_id
  sku_name                   = var.sku_name
  enable_rbac_authorization  = true
  purge_protection_enabled   = true
  soft_delete_retention_days = var.soft_delete_retention_days
  tags                       = local.base_tags
}

# The provisioning SP needs Key Vault Crypto Officer on the vault so the
# subsequent `azurerm_key_vault_key` create succeeds. Skip when caller doesn't
# want the CMK (e.g. they bring their own external key).
resource "azurerm_role_assignment" "sp_crypto_officer" {
  count                = var.create_cmk ? 1 : 0
  scope                = azurerm_key_vault.this.id
  role_definition_name = "Key Vault Crypto Officer"
  principal_id         = data.azurerm_client_config.current.object_id
}

resource "azurerm_key_vault_key" "cmk" {
  count        = var.create_cmk ? 1 : 0
  name         = local.key_name
  key_vault_id = azurerm_key_vault.this.id
  key_type     = "RSA"
  key_size     = 2048
  key_opts     = ["decrypt", "encrypt", "sign", "unwrapKey", "verify", "wrapKey"]
  depends_on   = [azurerm_role_assignment.sp_crypto_officer]
}
