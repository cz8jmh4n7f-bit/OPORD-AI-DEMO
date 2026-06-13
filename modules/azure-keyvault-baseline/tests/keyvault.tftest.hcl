# Companion: Key Vault baseline. Mocked azurerm + azuread, plan-only. Asserts
# the vault name (24-char cap), mandatory purge protection, and the create_cmk
# toggle gating the CMK + its role assignment.

mock_provider "azurerm" {
  mock_data "azurerm_client_config" {
    defaults = {
      tenant_id = "11111111-1111-1111-1111-111111111111"
      object_id = "22222222-2222-2222-2222-222222222222"
    }
  }
}
mock_provider "azuread" {}

variables {
  subscription_id  = "00000000-0000-0000-0000-000000000000"
  security_rg_name = "opord-acme-security-rg"
  location         = "westeurope"
  name_prefix      = "opord"
  csa_id           = "acme"
}

run "vault_named_and_protected" {
  command = plan

  assert {
    condition     = azurerm_key_vault.this.name == "opord-acme-kv"
    error_message = "vault name wrong"
  }
  assert {
    condition     = azurerm_key_vault.this.purge_protection_enabled == true
    error_message = "purge protection must be enabled (ADR-0009 mandatory)"
  }
  assert {
    condition     = length(azurerm_key_vault.this.name) <= 24
    error_message = "vault name must be <= 24 chars"
  }
}

run "cmk_disabled_by_default" {
  command = plan

  assert {
    condition     = length(azurerm_key_vault_key.cmk) == 0
    error_message = "create_cmk defaults false to no CMK"
  }
  assert {
    condition     = length(azurerm_role_assignment.sp_crypto_officer) == 0
    error_message = "no crypto-officer assignment when CMK is off"
  }
}

# Note: the create_cmk=true path isn't asserted offline - the CMK's role
# assignment validates `scope = key_vault.id`, and the mock provider returns a
# root ("/") id for the computed vault id rather than the supplied default, so
# the validator rejects it. The create_cmk gating is covered by the Go-side
# default (false) + the live 7/7 run; the offline test asserts the default-off
# path + naming + mandatory purge protection.
