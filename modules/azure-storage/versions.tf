# OPORD Azure Storage module: Storage Account + optional Blob container.
# Stack-style (no backend block; OPORD injects a workspace pg backend).
# Auth from ARM_TENANT_ID / ARM_CLIENT_ID / ARM_CLIENT_SECRET / ARM_SUBSCRIPTION_ID.

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
  features {}
}
