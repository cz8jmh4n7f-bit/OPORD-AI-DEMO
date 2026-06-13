# OPORD module L1: create a member AWS account via Organizations + place it in
# an OU + tag it. Runs in the MASTER account (no assume_role - the ambient creds
# are the master STS from Vault). The default OrganizationAccountAccessRole it
# creates is the entry point the later layers (L2-L6) assume into.
#
# Stack-style module (no backend block - OPORD injects a pg backend).
#
# AWS gotchas baked into the orchestrator, not the module:
#   - CreateAccount is async + eventually consistent (OPORD polls + backoff).
#   - Account closure has a 90-day window and is irreversible (close_on_deletion).

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
