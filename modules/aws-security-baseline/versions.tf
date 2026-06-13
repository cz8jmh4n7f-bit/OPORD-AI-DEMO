# OPORD module L5: security baseline for a member account - multi-region
# CloudTrail (KMS-encrypted, log-file validation) to S3, GuardDuty, and Security
# Hub with the AWS Foundational + CIS standards. Runs IN the member account via
# assume_role. (AWS Config recorder lives in L2 baseline; this layer adds the
# detective controls.)
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
    session_name = "opord-security-baseline"
  }
}
