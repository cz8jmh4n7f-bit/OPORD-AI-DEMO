# OPORD Azure subscription factory L1: adopt an existing subscription OR create
# a new one through MCA (Microsoft Customer Agreement) billing scope. Output
# subscription_id is consumed by every downstream layer (L2-L5 + companions).

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0"
    }
    time = {
      source  = "hashicorp/time"
      version = ">= 0.10"
    }
  }
}

provider "azurerm" {
  features {}
}
