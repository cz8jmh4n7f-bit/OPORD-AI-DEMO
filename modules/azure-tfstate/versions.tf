# OPORD Azure tfstate backend module: provisions the Storage Account that
# hosts OpenTofu state files for operators who require state-in-Azure for
# compliance/locality. Standalone - apply ONCE per Azure tenant before any
# other module starts writing state here. The Postgres pg backend (ADR-0003)
# stays OPORD's default; this is the opt-in alternative.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
  }
}

provider "azurerm" {
  # Soft-delete on the storage account itself is governed by the blob_properties
  # block in main.tf (delete_retention_policy + container_delete_retention_policy).
  # No special provider-level overrides needed.
  features {}
}
