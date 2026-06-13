# OPORD GCP database module: a Cloud SQL instance (PostgreSQL/MySQL) + a database
# + a user with a random password kept in state. Auth via GOOGLE_CREDENTIALS /
# GOOGLE_OAUTH_ACCESS_TOKEN + GOOGLE_PROJECT env.

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

provider "google" {}
