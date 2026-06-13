# OPORD original module - Kubernetes node provisioning on Proxmox VE.
# Clones a template into control-plane + worker VMs via bpg/proxmox and emits an
# Ansible inventory for the provider-agnostic kubeadm bootstrap (Phase 2).

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    proxmox = {
      source  = "bpg/proxmox"
      version = ">= 0.60.0"
    }
  }

  backend "pg" {}
}

provider "proxmox" {
  endpoint = var.proxmox_endpoint
  username = var.proxmox_username
  password = var.proxmox_password
  insecure = var.proxmox_insecure
}
