# OPORD GCP project factory - Layer "iam" (ADR-0011).
# A project custom role (least-privilege) + IAM bindings for EXISTING members
# (users/groups/service accounts). Never primitive roles (owner/editor) for
# humans - the role-name to predefined-role mapping happens in the Go provider.
# Layer 6 (Google Workspace user PROVISIONING is a later opt-in layer).

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
