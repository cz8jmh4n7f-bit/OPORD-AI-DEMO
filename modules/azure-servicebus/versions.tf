# OPORD Azure Service Bus module: Standard-tier namespace + queue(s).
# Service Bus is the rough analog of AWS SQS (basic) + SNS (FIFO/topic).
# Standard tier supports basic queues + topics; Premium tier adds VNet
# integration + larger message size (256 KB to 100 MB).

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
