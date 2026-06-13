# OPORD GCP secret module: a Secret Manager secret CONTAINER (no version/value -
# the plaintext is set out-of-band, so OPORD never holds it). Auth via
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
