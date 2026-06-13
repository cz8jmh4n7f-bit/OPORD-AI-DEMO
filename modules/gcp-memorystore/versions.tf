# OPORD GCP cache module: a Memorystore for Redis instance (the "cache"
# primitive). BASIC tier for a single node, STANDARD_HA when replicated. Auth via
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
