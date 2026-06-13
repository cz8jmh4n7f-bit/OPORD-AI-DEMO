# OPORD original module - written from public OpenTofu/vSphere provider docs.
# Provisions VMs for a Kubernetes cluster on VMware vSphere (Phase 1: infra only).

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    vsphere = {
      source  = "hashicorp/vsphere"
      version = "~> 2.8"
    }
  }

  # State lives in the app Postgres via the pg backend, one workspace per cluster
  # (see docs/adr/0003-state-isolation.md). Connection is supplied at init time:
  #   tofu init -backend-config="conn_str=postgres://..."
  backend "pg" {}
}

provider "vsphere" {
  vsphere_server       = var.vsphere_server
  user                 = var.vsphere_user
  password             = var.vsphere_password
  allow_unverified_ssl = var.vsphere_allow_unverified_ssl
}
