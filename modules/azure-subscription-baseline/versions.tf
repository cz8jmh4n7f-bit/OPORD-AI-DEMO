# OPORD Azure subscription factory L2: baseline of the subscription itself.
# Registers the resource providers later layers need, enables Microsoft
# Defender for Cloud (Free tier - CSPM only, $0/mo), and provisions the
# three foundational resource groups every layer/runbook references.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
  }
}

# Multi-subscription mode: this module operates on a different subscription
# than the one the SP defaults to (the new one L1 just provisioned, or the
# adopted one). The alias is used by every resource in this module.
provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
}
