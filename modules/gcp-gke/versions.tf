# OPORD GCP managed Kubernetes module: a GKE Standard cluster (zonal for cost)
# with the default node pool removed and a dedicated node pool. Managed control
# plane - OPORD skips the kubeadm (Ansible) bootstrap. Auth via GOOGLE_CREDENTIALS
# / GOOGLE_OAUTH_ACCESS_TOKEN + GOOGLE_PROJECT env.

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
