# OPORD GCP queue module: a Pub/Sub topic + a pull subscription (the "queue"
# primitive maps onto Pub/Sub). Optional dead-letter topic + dead_letter_policy.
# Auth via GOOGLE_CREDENTIALS / GOOGLE_OAUTH_ACCESS_TOKEN + GOOGLE_PROJECT env.

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
