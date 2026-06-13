# OPORD Azure subscription factory companion: preventive guardrails via
# Azure Policy. Assigns a curated set of built-in policy definitions at the
# subscription scope. V1 ships the minimum set the runbook calls out:
# allowed locations + audit non-managed disks + KV purge protection enforced
# + storage account network access restriction.

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
