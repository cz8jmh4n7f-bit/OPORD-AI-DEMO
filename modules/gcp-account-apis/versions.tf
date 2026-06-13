# OPORD GCP project factory - Layer "apis" (ADR-0011).
# Enables the required project services, then waits for enablement to propagate
# (GCP API enablement is eventually consistent - creating a resource that uses a
# just-enabled API too soon fails). disable_on_destroy=false avoids API churn.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.0"
    }
    time = {
      source  = "hashicorp/time"
      version = ">= 0.9"
    }
  }
}

provider "google" {}
