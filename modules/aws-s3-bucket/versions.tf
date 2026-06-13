# OPORD first-class module: S3 bucket with secure defaults (block-public-access,
# versioning, server-side encryption). Stack-style - NO backend block; OPORD
# injects a workspace pg backend. Cost: storage + requests only.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}
