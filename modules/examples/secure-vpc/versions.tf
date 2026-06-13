# Standalone variant of modules/aws-secure-vpc (L4): same VPC topology, but a
# PLAIN aws provider (no cross-account assume_role). OPORD runs it directly in
# the account the resolved credentials belong to - used to test the secure-VPC
# module against a single account without the account factory.
#
# Stack-style module: NO backend block (OPORD injects a workspace pg backend).
# Cost: ~$0 - VPC/subnets/IGW/route-tables/SG are free; Flow Logs to CloudWatch
# with no traffic are negligible. No NAT gateways / VPC endpoints / EIPs.

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
