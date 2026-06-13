# OPORD first-class module: SQS queue (standard or FIFO) with optional DLQ,
# at-rest encryption, and long-polling defaults. Stack-style - NO backend block.
# Cost: $0.40 per million requests after the 1M free tier; no fixed cost.

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
