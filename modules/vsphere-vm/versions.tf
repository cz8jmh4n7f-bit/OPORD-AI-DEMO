# OPORD original module - standalone VM provisioning on VMware vSphere.
# Clones a golden template (built by the user's Packer) into N plain VMs.
# No Kubernetes; this is the generic "vm" blueprint.

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    vsphere = {
      source  = "hashicorp/vsphere"
      version = "~> 2.8"
    }
  }

  # State in the app Postgres via pg backend, one workspace per resource.
  # tofu init -backend-config="conn_str=postgres://..."
  backend "pg" {}
}

provider "vsphere" {
  vsphere_server       = var.vsphere_server
  user                 = var.vsphere_user
  password             = var.vsphere_password
  allow_unverified_ssl = var.vsphere_allow_unverified_ssl
}
