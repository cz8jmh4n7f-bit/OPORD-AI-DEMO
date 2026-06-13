# OPORD Azure subscription factory L5: centralised security telemetry.
# Log Analytics Workspace, CMK-encrypted Storage Account for long-term archive
# (consumed by L4 Flow Logs too), Diagnostic Setting forwarding the
# subscription-scope Activity Log to BOTH the LAW and the Storage Account.

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
