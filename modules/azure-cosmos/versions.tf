# OPORD Azure Cosmos DB module: SQL (Core) API account + database + container.
# Cosmos has 5 API surfaces; SQL is the default and maps cleanly onto the
# DynamoDB-style hash/range key model OPORD exposes via TableSpec.

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
