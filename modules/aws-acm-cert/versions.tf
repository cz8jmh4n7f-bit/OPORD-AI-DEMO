# OPORD original module - ACM certificate (DNS-validated) for the expose layer.
# AWS credentials and region come from the ambient environment (OPORD injects
# AWS_REGION; us-east-1-for-CloudFront is handled by the caller, not here).

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
