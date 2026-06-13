# OPORD module L3: human access to the member account via SAML federation
# (Azure AD / Entra ID) + custom IAM roles (Admin / Manager / ReadOnly). Runs IN
# the member account via assume_role.
#
# Alternative to this module: vend access through IAM Identity Center with the
# `project` primitive (modules/aws-sso-project) - pick SAML *or* Identity Center
# per the org's identity model (DR-2). This module is the SAML path.
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
    session_name = "opord-account-access"
  }
}
