# OPORD original module - expose-layer: AWS API Gateway v2 (HTTP API).
# Fronts a Lambda function or an upstream HTTP service with an optional custom
# domain. AWS credentials and region come from the ambient environment that
# OPORD injects (AWS_REGION), so the provider region is not pinned here.

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
