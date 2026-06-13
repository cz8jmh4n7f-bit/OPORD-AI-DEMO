# OPORD original module - AWS Route53 hosted zone (expose-layer, ADR-0016).
# Creates a public (or VPC-private) hosted zone, optionally with records.
# AWS credentials and region come from the ambient environment (AWS_REGION
# injected by OPORD) - no provider block pins the region.

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
