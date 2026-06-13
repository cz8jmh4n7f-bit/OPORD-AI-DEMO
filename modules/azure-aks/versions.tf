# OPORD Azure Kubernetes Service module: managed AKS cluster with a
# system node pool. Outputs include the kubeconfig in raw form so OPORD can
# write it to a local file (analogous to the aws-eks pattern).

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
