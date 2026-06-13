# OPORD Azure PostgreSQL Flexible Server module: managed Postgres in a
# dedicated resource group, public-access flexible server (no VNet
# integration in Phase 2 - that's a follow-up). OPORD never stores the
# admin password; we set a random one and surface it ONLY via the connection
# string output that points at Azure-managed credentials in Key Vault... but
# Azure Flexible Server requires us to set the password at create time, so the
# password is in tofu state. Treat the workspace state as a secret.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.5"
    }
  }
}

provider "azurerm" {
  features {}
}
