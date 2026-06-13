# OPORD module L2: account hardening baseline - strong IAM password policy,
# account-wide S3 public-access block, EBS encryption-by-default, AWS Config
# recorder, and a monthly budget alert. Runs IN the member account via
# assume_role.
#
# NOTE on default-VPC deletion: removing the default VPC in *every* region in
# pure Terraform needs an aliased provider per region (~17), which is fragile.
# OPORD does this sweep via the AWS API in the orchestrator (account_layers.go,
# idempotent) - see runbook 01-onboarding. This module covers the declarative
# baseline.
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
  assume_role {
    role_arn     = var.assume_role_arn
    session_name = "opord-account-baseline"
  }
}
