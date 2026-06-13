# OPORD GCP project factory - Layer "vpc" (ADR-0011).
# A secure VPC (no auto subnets) + 3 /24 subnets carved from a /22 (allocated by
# OPORD's IPAM from the OpenBao CIDR pool) with Private Google Access + VPC flow
# logs, plus a Zero-Trust (ZTNA) firewall: explicit allow for trusted sources,
# deny everything else, ingress AND egress. Layer 4.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.0"
    }
  }
}

provider "google" {}
