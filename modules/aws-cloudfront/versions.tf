# OPORD original module - CloudFront distribution (CDN / expose-layer, ADR-0016).
# Fronts an origin (S3 website, ALB, API Gateway, or a custom HTTP origin) with a
# CloudFront distribution. CloudFront is a GLOBAL service - the provider region
# still comes from var.region (or the ambient AWS_REGION OPORD injects), but the
# distribution itself is global. When aliases/HTTPS on a custom domain are used,
# the ACM certificate MUST live in us-east-1; the caller guarantees that region.
# AWS credentials come from the ambient environment.

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
