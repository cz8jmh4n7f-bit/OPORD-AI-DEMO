# OPORD GCP project factory - Layer "org-policy" (ADR-0011).
# Project-level Organization Policy constraints (v2 orgpolicy API). Boolean
# guardrails enforced by default; list constraints (allowed locations / member
# domains) applied only when provided. Layer 5.

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
