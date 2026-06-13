# OPORD GCP project factory - Layer "security" (ADR-0011).
# KMS keyring + crypto key (rotated) to a CMEK-encrypted log-sink bucket + a
# project log sink + log-based metrics for critical events. Layer 3.

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
