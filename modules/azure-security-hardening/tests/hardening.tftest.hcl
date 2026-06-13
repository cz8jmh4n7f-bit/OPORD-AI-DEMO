# L5 security hardening: mocked azurerm, plan-only. Asserts LAW + archive SA
# naming and the CMK opt-in gating.

mock_provider "azurerm" {
  mock_resource "azurerm_storage_account" {
    defaults = {
      id = "/subscriptions/x/resourceGroups/y/providers/Microsoft.Storage/storageAccounts/s"
    }
  }
  mock_resource "azurerm_log_analytics_workspace" {
    defaults = {
      id = "/subscriptions/x/resourceGroups/y/providers/Microsoft.OperationalInsights/workspaces/w"
    }
  }
}

variables {
  subscription_id          = "00000000-0000-0000-0000-000000000000"
  subscription_resource_id = "/subscriptions/00000000-0000-0000-0000-000000000000"
  logs_rg_name             = "opord-acme-logs-rg"
  location                 = "westeurope"
  name_prefix              = "opord"
  csa_id                   = "acme"
}

run "law_and_archive_named" {
  command = plan

  assert {
    condition     = azurerm_log_analytics_workspace.this.name == "opord-acme-logs-law"
    error_message = "LAW name wrong"
  }
  assert {
    condition     = azurerm_storage_account.archive.name == "opordacmelogssa"
    error_message = "archive SA name wrong (must match archiveStorageAccountID in Go)"
  }
}

run "cmk_off_by_default" {
  command = plan

  assert {
    condition     = length(azurerm_storage_account_customer_managed_key.cmk) == 0
    error_message = "no CMK wiring when cmk_versionless_id is empty"
  }
}

run "cmk_wired_when_supplied" {
  command = plan

  variables {
    cmk_versionless_id = "https://opord-acme-kv.vault.azure.net/keys/acme-cmk"
  }

  assert {
    condition     = length(azurerm_storage_account_customer_managed_key.cmk) == 1
    error_message = "CMK should be wired when a versionless id is supplied"
  }
}
