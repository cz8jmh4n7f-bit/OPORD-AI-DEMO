# OPORD first-class module: a Secrets Manager secret (KMS-encrypted at rest),
# optionally tied to a rotation Lambda. The secret VALUE is NOT set here -
# OPORD creates the container and ARN; you populate the value out-of-band
# (CLI/console/sync-from-Vault) so plaintext never crosses the OPORD API.
#
# Stack-style - NO backend block; OPORD injects a workspace pg backend.
# Cost: $0.40/secret/month + $0.05 per 10k API calls.

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
