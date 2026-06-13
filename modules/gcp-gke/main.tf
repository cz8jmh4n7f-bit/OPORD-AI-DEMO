variable "name" {
  type = string
}

variable "region" {
  type    = string
  default = "europe-west1"
}

variable "zone" {
  type        = string
  default     = ""
  description = "Zone for a zonal cluster; defaults to <region>-b."
}

variable "kubernetes_version" {
  type    = string
  default = ""
}

variable "node_count" {
  type    = number
  default = 2
}

variable "machine_type" {
  type    = string
  default = "e2-small"
}

variable "disk_gb" {
  type    = number
  default = 30
}

variable "environment" {
  type    = string
  default = "dev"
}

variable "network" {
  type        = string
  default     = ""
  description = "VPC self-link/name. Empty = the project's 'default' network. Required for a governed project (auto_create_network=false) that has no default."
}

variable "subnetwork" {
  type        = string
  default     = ""
  description = "Subnetwork self-link/name in the cluster region. Empty = auto. Required alongside a custom network."
}

variable "cni" {
  type        = string
  default     = ""
  description = "CNI choice. 'cilium' enables GKE Dataplane V2 (ADVANCED_DATAPATH, Cilium-based). Anything else uses GKE's default datapath."
}

locals {
  location = var.zone != "" ? var.zone : "${var.region}-b"
  # GKE is a managed control plane: the only datapath that maps to a real CNI choice
  # is Dataplane V2 (Cilium). 'cilium' -> ADVANCED_DATAPATH; otherwise leave GKE's default.
  datapath_provider = lower(var.cni) == "cilium" ? "ADVANCED_DATAPATH" : null
}

resource "google_container_cluster" "this" {
  name                     = var.name
  location                 = local.location
  remove_default_node_pool = true
  initial_node_count       = 1
  deletion_protection      = false
  min_master_version       = var.kubernetes_version != "" ? var.kubernetes_version : null

  # CNI=cilium -> GKE Dataplane V2 (Cilium). null = GKE default datapath.
  datapath_provider = local.datapath_provider

  # Explicit network/subnetwork for governed projects that have NO "default" network
  # (the account factory sets auto_create_network=false). Empty = GKE's default
  # behaviour (the project's "default" network) for an ordinary project.
  network    = var.network != "" ? var.network : null
  subnetwork = var.subnetwork != "" ? var.subnetwork : null
}

resource "google_container_node_pool" "this" {
  name       = "default"
  cluster    = google_container_cluster.this.name
  location   = local.location
  node_count = var.node_count

  node_config {
    machine_type = var.machine_type
    disk_size_gb = var.disk_gb
    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
    labels = {
      opord_env = var.environment
    }
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }
}

output "endpoint" {
  value       = google_container_cluster.this.endpoint
  description = "The control plane endpoint (IP/host)."
}

output "cluster_name" {
  value = google_container_cluster.this.name
}

output "location" {
  value = local.location
}

output "ca_certificate" {
  value     = google_container_cluster.this.master_auth[0].cluster_ca_certificate
  sensitive = true
}
