# OPORD original module - standalone VM (EC2 instance) provisioning on AWS.
# Launches N instances from an AMI. Generic "vm" blueprint for AWS.
# AWS credentials come from the ambient environment (AWS_ACCESS_KEY_ID/SECRET or
# a shared profile) read by the provider.

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
