variable "project_id" {
  type = string
}

variable "csa_id" {
  type = string
}

variable "region" {
  type    = string
  default = "europe-west1"
}

variable "vpc_cidr" {
  type        = string
  description = "A /22 (from OPORD IPAM) carved into 3 /24 subnets."
  validation {
    condition     = can(regex("/22$", var.vpc_cidr))
    error_message = "vpc_cidr must be a /22."
  }
}

variable "allow_inbound_cidrs" {
  type        = list(string)
  default     = ["0.0.0.0/0"]
  description = "Trusted source CIDRs (org IP ranges) for the ZTNA allow rules. Dev default 0.0.0.0/0."
}

variable "subnet_count" {
  type    = number
  default = 3
}

locals {
  vpc_name = "opord-${var.csa_id}-vpc"
  subnets = [for i in range(var.subnet_count) : {
    name = "opord-${var.csa_id}-subnet-${i}"
    cidr = cidrsubnet(var.vpc_cidr, 2, i)
  }]
  # Restricted Google APIs ranges (Private Google Access / restricted.googleapis.com).
  googleapis_ranges = ["199.36.153.4/30", "199.36.153.8/30"]
}

resource "google_compute_network" "this" {
  name                    = local.vpc_name
  project                 = var.project_id
  auto_create_subnetworks = false
  routing_mode            = "REGIONAL"
}

resource "google_compute_subnetwork" "this" {
  for_each                 = { for s in local.subnets : s.name => s }
  name                     = each.value.name
  project                  = var.project_id
  region                   = var.region
  network                  = google_compute_network.this.id
  ip_cidr_range            = each.value.cidr
  private_ip_google_access = true

  log_config {
    aggregation_interval = "INTERVAL_5_SEC"
    flow_sampling        = 0.5
    metadata             = "INCLUDE_ALL_METADATA"
  }
}

# --- ZTNA firewall (ingress: explicit allow from trusted, then deny) ---

resource "google_compute_firewall" "allow_ssh" {
  name          = "opord-${var.csa_id}-allow-ssh"
  project       = var.project_id
  network       = google_compute_network.this.name
  priority      = 100
  direction     = "INGRESS"
  source_ranges = var.allow_inbound_cidrs
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

resource "google_compute_firewall" "allow_rdp" {
  name          = "opord-${var.csa_id}-allow-rdp"
  project       = var.project_id
  network       = google_compute_network.this.name
  priority      = 200
  direction     = "INGRESS"
  source_ranges = var.allow_inbound_cidrs
  allow {
    protocol = "tcp"
    ports    = ["3389"]
  }
}

resource "google_compute_firewall" "allow_https" {
  name          = "opord-${var.csa_id}-allow-https"
  project       = var.project_id
  network       = google_compute_network.this.name
  priority      = 300
  direction     = "INGRESS"
  source_ranges = var.allow_inbound_cidrs
  allow {
    protocol = "tcp"
    ports    = ["443"]
  }
}

resource "google_compute_firewall" "allow_icmp" {
  name          = "opord-${var.csa_id}-allow-icmp"
  project       = var.project_id
  network       = google_compute_network.this.name
  priority      = 400
  direction     = "INGRESS"
  source_ranges = var.allow_inbound_cidrs
  allow {
    protocol = "icmp"
  }
}

resource "google_compute_firewall" "deny_all_ingress" {
  name          = "opord-${var.csa_id}-deny-all-ingress"
  project       = var.project_id
  network       = google_compute_network.this.name
  priority      = 500
  direction     = "INGRESS"
  source_ranges = ["0.0.0.0/0"]
  deny {
    protocol = "all"
  }
}

# --- ZTNA firewall (egress: allow to org + Google APIs, then deny) ---

resource "google_compute_firewall" "allow_egress_to_org" {
  name               = "opord-${var.csa_id}-allow-egress-org"
  project            = var.project_id
  network            = google_compute_network.this.name
  priority           = 600
  direction          = "EGRESS"
  destination_ranges = var.allow_inbound_cidrs
  allow {
    protocol = "all"
  }
}

resource "google_compute_firewall" "allow_egress_googleapis" {
  name               = "opord-${var.csa_id}-allow-egress-gapis"
  project            = var.project_id
  network            = google_compute_network.this.name
  priority           = 650
  direction          = "EGRESS"
  destination_ranges = local.googleapis_ranges
  allow {
    protocol = "tcp"
    ports    = ["443"]
  }
}

resource "google_compute_firewall" "deny_all_egress" {
  name               = "opord-${var.csa_id}-deny-all-egress"
  project            = var.project_id
  network            = google_compute_network.this.name
  priority           = 700
  direction          = "EGRESS"
  destination_ranges = ["0.0.0.0/0"]
  deny {
    protocol = "all"
  }
}

output "network" {
  value = google_compute_network.this.id
}

output "network_name" {
  value = google_compute_network.this.name
}

output "subnets" {
  value = { for k, s in google_compute_subnetwork.this : k => s.ip_cidr_range }
}

output "vpc_cidr" {
  value = var.vpc_cidr
}
