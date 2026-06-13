# OPORD Azure Cache for Redis module: a managed Redis instance in its own
# resource group, TLS-only. Auth via ARM_* env vars; pg backend injected by
# OPORD. The Azure analog of modules/aws-elasticache.

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
