# OPORD Azure access-vending module: an Entra security group granted an Azure
# RBAC role at a subscription / resource-group scope, with members. The Azure
# analog of modules/aws-sso-project. Auth via ARM_* env vars (shared by the
# azurerm + azuread providers); pg backend injected by OPORD.

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
}

provider "azuread" {}
