# OPORD original module - managed relational database (RDS) on AWS.
# Creates a single RDS instance in a subnet group. The master password is
# managed by RDS in Secrets Manager (manage_master_user_password) so OPORD never
# handles a plaintext password. AWS credentials come from the ambient environment.

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
