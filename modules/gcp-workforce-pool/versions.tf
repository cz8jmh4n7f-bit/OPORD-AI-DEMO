# OPORD GCP Workforce Identity Federation pool (ADR-0012).
terraform {
  required_version = ">= 1.6"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.30"
    }
  }
}
