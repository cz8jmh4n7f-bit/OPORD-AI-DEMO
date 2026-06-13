# OPORD Azure subscription factory companion: a Key Vault per subscription
# that L4 (secure-vnet Flow Logs SA) and L5 (security-hardening logs archive)
# consume for Customer-Managed Keys. Soft delete + purge protection are
# MANDATORY: deleting a KV with the same name within 90 days blocks recreate.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
    azuread = {
      source  = "hashicorp/azuread"
      version = ">= 3.0"
    }
  }
}

provider "azurerm" {
  features {
    key_vault {
      # MUST be false: the vault sets purge_protection_enabled=true (mandatory,
      # ADR-0009), which makes purge impossible for the soft-delete retention
      # window. With purge_soft_delete_on_destroy=true the destroy tries to
      # purge anyway, Azure 409s ("purge protection enabled"), the KV layer
      # destroy errors, the whole account destroy fails, and River retries
      # forever - the destroy never converges. With false, destroy soft-deletes
      # the vault and SUCCEEDS; the name is blocked for the retention window, so
      # re-provisioning within it needs a fresh csa_id (documented in the
      # decommissioning runbook). This is the correct prod behaviour anyway.
      purge_soft_delete_on_destroy    = false
      recover_soft_deleted_key_vaults = false
    }
  }
  subscription_id = var.subscription_id
}

provider "azuread" {}
