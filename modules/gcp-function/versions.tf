# OPORD GCP function module: a 2nd-gen Cloud Function (Cloud Run-based). When no
# external source is given, a built-in "hello" handler is zipped + uploaded to a
# source bucket so the function is immediately deployable. Auth via
# GOOGLE_CREDENTIALS / GOOGLE_OAUTH_ACCESS_TOKEN + GOOGLE_PROJECT env.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.0"
    }
    archive = {
      source  = "hashicorp/archive"
      version = ">= 2.4"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.5"
    }
  }
}

provider "google" {}
