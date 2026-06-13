# OPORD Azure VM module: Linux VM in a dedicated resource group with a NIC,
# a Public IP, and a minimal NSG (SSH-from-anywhere only). Stack-style - NO
# backend block; OPORD injects a workspace pg backend.
#
# Auth: the `azurerm` provider reads ARM_TENANT_ID / ARM_CLIENT_ID /
# ARM_CLIENT_SECRET / ARM_SUBSCRIPTION_ID from the environment - OPORD sets
# them from the provider's resolved credentials (OpenBao path opord/azure/...).

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
