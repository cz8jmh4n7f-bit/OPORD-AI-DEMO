# OPORD original module - managed Kubernetes (EKS) on AWS.
# Creates an EKS control plane + a managed node group, with the IAM roles each
# requires. Subnets are supplied (this module does not create a VPC). AWS
# credentials come from the ambient environment read by the provider.

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }

  backend "pg" {}
}

provider "aws" {
  region = var.region
}
