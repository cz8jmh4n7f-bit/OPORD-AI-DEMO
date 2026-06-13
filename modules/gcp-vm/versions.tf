# OPORD GCP VM module: a Compute Engine instance in its own VPC + subnet with a
# locked firewall and an optional external IP. Auth via GOOGLE_CREDENTIALS +
# GOOGLE_PROJECT env (injected by OPORD); pg backend injected per workspace.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.0"
    }
  }
}

# project comes from GOOGLE_PROJECT; region/zone are passed so the module is
# self-contained for plan/validate.
provider "google" {
  region = var.region
  zone   = var.zone
}
