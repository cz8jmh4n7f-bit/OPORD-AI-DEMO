# OPORD Azure subscription factory L3: custom RBAC roles + Entra groups +
# group-to-role assignments on the subscription scope. PIM-safe: assignments
# go to GROUPS, not users - direct user assignment conflicts with PIM
# eligibility. Group membership is managed by OPORD's internal/azure Graph
# client (item 49), out-of-band from this module.

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
  features {}
  subscription_id = var.subscription_id
}

provider "azuread" {}
