# Example stack: SAML federation into THIS account (Azure AD / Entra ID to AWS).
#
# Same resources as modules/aws-account-access (L3) - an IAM SAML provider + a
# set of console roles assumable via SAML - but with a PLAIN aws provider (no
# cross-account assume_role). OPORD runs it directly in the account the resolved
# credentials belong to. Use it to test the Entra to AWS round-trip against a
# single account (e.g. the management account) without the account factory.
#
# Stack-style module: NO backend block (OPORD injects a workspace pg backend).
# Cost: $0 - IAM SAML providers and roles are free.

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

locals {
  base_tags = merge(var.tags, { ManagedBy = "opord", Purpose = "saml-federation" })
  # Flatten roles to (role, policy_arn) pairs for managed-policy attachments.
  role_policies = merge([
    for role, arns in var.roles : {
      for arn in arns : "${role}::${arn}" => { role = role, arn = arn }
    }
  ]...)
}

# Azure AD / Entra ID as a SAML identity provider in this account.
resource "aws_iam_saml_provider" "azuread" {
  name                   = "${var.name}-azuread"
  saml_metadata_document = file(var.saml_metadata_path)
  tags                   = local.base_tags
}

# Trust policy: allow federated sign-in from the SAML provider to the AWS console.
data "aws_iam_policy_document" "saml_assume" {
  statement {
    actions = ["sts:AssumeRoleWithSAML"]
    principals {
      type        = "Federated"
      identifiers = [aws_iam_saml_provider.azuread.arn]
    }
    condition {
      test     = "StringEquals"
      variable = "SAML:aud"
      values   = ["https://signin.aws.amazon.com/saml"]
    }
  }
}

resource "aws_iam_role" "this" {
  for_each             = var.roles
  name                 = "${var.name}-${each.key}"
  assume_role_policy   = data.aws_iam_policy_document.saml_assume.json
  max_session_duration = var.session_duration
  tags                 = merge(local.base_tags, { Role = each.key })
}

resource "aws_iam_role_policy_attachment" "this" {
  for_each   = local.role_policies
  role       = aws_iam_role.this[each.value.role].name
  policy_arn = each.value.arn
}
