# OPORD "stack" module: AWS access-vending via IAM Identity Center (SSO).
#
# A "project" = one Identity Center GROUP, bound to a permission set on an
# EXISTING AWS account, with the requested existing users added as members.
# "Add a user later" = append to var.user_names and re-apply (idempotent).
#
# This is a stack module: it does NOT declare a backend (OPORD injects a
# workspace-isolated pg backend). Credentials come from the ambient AWS_* env
# that OPORD injects from the resolved provider creds (OpenBao/env) - the
# provider must point at the org's Identity Center management/delegated-admin
# account.

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
