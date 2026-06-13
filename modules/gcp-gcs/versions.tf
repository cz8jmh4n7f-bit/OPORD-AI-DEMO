# OPORD GCP object storage module: a Cloud Storage bucket with uniform
# bucket-level access, versioning, and enforced public-access prevention.
# Auth via GOOGLE_CREDENTIALS / GOOGLE_OAUTH_ACCESS_TOKEN + GOOGLE_PROJECT env
# (injected by OPORD); pg backend injected per workspace.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.5"
    }
  }
}

# project comes from GOOGLE_PROJECT (env, injected by OPORD).
provider "google" {}
