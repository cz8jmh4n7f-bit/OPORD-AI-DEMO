# OPORD module L4: a secure VPC (/22) with 3 private-by-default /24 subnets
# across AZs, IGW, locked-down default SG, and VPC Flow Logs to CloudWatch.
# Runs IN the member account via assume_role into its bootstrap role.
#
# The /22 CIDR is allocated atomically from the Vault pool by OPORD's IPAM
# (internal/ipam, KV v2 CAS) and passed in as var.vpc_cidr.
#
# Stack-style module (no backend block - OPORD injects a pg backend).

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
  # Cross-account: assume the member account's bootstrap role (DR-3). The
  # ambient creds are the master STS; this hops into the target account.
  assume_role {
    role_arn     = var.assume_role_arn
    session_name = "opord-secure-vpc"
  }
}
