# OPORD GCP table module: a Firestore database (the GCP managed NoSQL, the
# closest analog to the provider-neutral "table" primitive). Firestore is
# schemaless, so the DynamoDB-shaped hash/range keys don't apply - the primitive
# provisions the managed database container. Auth via GOOGLE_CREDENTIALS /
# GOOGLE_OAUTH_ACCESS_TOKEN + GOOGLE_PROJECT env.

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
