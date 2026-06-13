# OPORD original module - a manual RDS snapshot of an existing DB instance.
# Run in its own workspace per backup. Destroying the workspace deletes the snapshot.

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
