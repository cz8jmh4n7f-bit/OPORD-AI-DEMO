locals {
  base_labels  = merge(var.labels, { environment = var.environment, managed_by = "opord" })
  ssh_metadata = var.ssh_public_key == "" ? {} : { "ssh-keys" = "${var.ssh_user}:${var.ssh_public_key}" }
}

resource "google_compute_network" "vpc" {
  name                    = "${var.name_prefix}-vpc"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "subnet" {
  name          = "${var.name_prefix}-subnet"
  ip_cidr_range = "10.10.0.0/24"
  region        = var.region
  network       = google_compute_network.vpc.id
}

# Locked firewall: SSH only, from the allowed source ranges. All other inbound
# is denied by GCP's implicit deny.
resource "google_compute_firewall" "ssh" {
  name          = "${var.name_prefix}-allow-ssh"
  network       = google_compute_network.vpc.id
  source_ranges = var.allow_ssh_from

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

resource "google_compute_instance" "vm" {
  count        = var.vm_count
  name         = "${var.name_prefix}-${count.index}"
  machine_type = var.machine_type
  zone         = var.zone
  labels       = local.base_labels

  boot_disk {
    initialize_params {
      image = var.image
      size  = var.disk_gb
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.subnet.id

    dynamic "access_config" {
      for_each = var.public_ip ? [1] : []
      content {}
    }
  }

  metadata = local.ssh_metadata
}
