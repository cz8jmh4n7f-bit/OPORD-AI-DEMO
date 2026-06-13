# OPORD Azure Key Vault module: standalone vault for secrets + keys.
# Auth via ARM_* env vars; pg backend injected by OPORD.

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
  features {
    key_vault {
      # purge_protection_enabled below decides whether deletes are soft or hard;
      # set this to NOT purge on tofu destroy so destroyed vaults disappear cleanly.
      purge_soft_delete_on_destroy = true
    }
  }
}
