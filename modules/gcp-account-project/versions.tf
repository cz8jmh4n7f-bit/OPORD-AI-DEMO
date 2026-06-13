# OPORD GCP project factory - Layer "project" (ADR-0011).
# A per-CSA folder under the project-factory parent + a GCP project with billing
# linked and NO default network (auto_create_network = false - a default network
# is a security nightmare). Auth via GOOGLE_CREDENTIALS / GOOGLE_OAUTH_ACCESS_TOKEN
# from the org-level provisioning identity; pg backend injected per workspace.

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
