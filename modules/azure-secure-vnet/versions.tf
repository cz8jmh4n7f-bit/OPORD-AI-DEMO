# OPORD Azure subscription factory L4: a /22 VNet with 3 /24 subnets, a deny-
# by-default NSG with explicit allow rules for trusted CIDRs, and VNet Flow
# Logs writing to a CMK-encrypted Storage Account. The CIDR is allocated
# atomically from the Vault pool opord-azure-vnet-cidr-pools by the Go
# orchestrator BEFORE this module runs; the module just consumes it via the
# vnet_cidr variable.

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
  subscription_id = var.subscription_id
}
