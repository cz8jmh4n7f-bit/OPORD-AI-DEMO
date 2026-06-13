# OPORD Azure Functions module: Linux Function App (Consumption plan = serverless,
# scales to 0). Includes the required Storage Account + Service Plan.

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
