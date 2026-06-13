# OPORD bootstrap module: org-level guardrails (Service Control Policies + tag
# policy). Runs ONCE in the Organizations management account - preventive
# controls that no member account (or its provisioning role) can override.
#
# Stack-style module (no backend block - OPORD injects a workspace-isolated pg
# backend). Master-account credentials come from the ambient AWS_* env that
# OPORD injects from the Vault AWS Secrets Engine (short-lived STS).
#
# Prereq: Organizations "all features" enabled, and the SERVICE_CONTROL_POLICY /
# TAG_POLICY policy types enabled on the root (see runbook 04-bootstrap).

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
