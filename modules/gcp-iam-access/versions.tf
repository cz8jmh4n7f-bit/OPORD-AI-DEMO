# OPORD GCP access module: grant an IAM role on a project to a set of members
# (the provider-neutral "project / access" primitive). Members are bound
# directly via google_project_iam_member (additive, non-authoritative). Auth via
# GOOGLE_CREDENTIALS / GOOGLE_OAUTH_ACCESS_TOKEN + GOOGLE_PROJECT env.

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
