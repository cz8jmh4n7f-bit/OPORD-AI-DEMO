locals {
  cp_ip_parts  = split(".", var.cp_ip_start)
  cp_ip_prefix = join(".", slice(local.cp_ip_parts, 0, 3))
  cp_ip_last   = tonumber(element(local.cp_ip_parts, 3))

  wk_ip_parts  = split(".", var.worker_ip_start)
  wk_ip_prefix = join(".", slice(local.wk_ip_parts, 0, 3))
  wk_ip_last   = tonumber(element(local.wk_ip_parts, 3))

  control_plane_nodes = [
    for i in range(var.control_plane_count) : {
      name = format("%s-%02d", var.cp_name_prefix, i + 1)
      ip   = format("%s.%d", local.cp_ip_prefix, local.cp_ip_last + i)
    }
  ]

  worker_nodes = [
    for i in range(var.worker_count) : {
      name = format("%s-%02d", var.worker_name_prefix, i + 1)
      ip   = format("%s.%d", local.wk_ip_prefix, local.wk_ip_last + i)
    }
  ]

  ansible_inventory = templatefile("${path.module}/templates/inventory.tftpl", {
    control_plane = local.control_plane_nodes
    workers       = local.worker_nodes
    ssh_user      = var.ssh_user
    cp_host       = var.control_plane_endpoint
    cp_port       = var.control_plane_endpoint_port
  })
}

resource "proxmox_virtual_environment_vm" "control_plane" {
  count = var.control_plane_count

  name      = local.control_plane_nodes[count.index].name
  node_name = var.node_name
  tags      = [var.environment, "opord", "control-plane"]

  clone {
    vm_id = var.template_vmid
    full  = true
  }

  agent {
    enabled = true
  }

  cpu {
    cores = var.control_plane_specs.cpu
    type  = "host"
  }

  memory {
    dedicated = var.control_plane_specs.memory
  }

  disk {
    datastore_id = var.datastore_id
    interface    = "scsi0"
    size         = var.control_plane_specs.disk
  }

  network_device {
    bridge = var.network_bridge
  }

  initialization {
    dns {
      domain  = var.dns_domain
      servers = var.dns_servers
    }
    ip_config {
      ipv4 {
        address = "${local.control_plane_nodes[count.index].ip}/${var.netmask_bits}"
        gateway = var.gateway
      }
    }
    user_account {
      username = var.ssh_user
      keys     = var.ssh_public_key != "" ? [var.ssh_public_key] : []
    }
  }

  lifecycle {
    ignore_changes = [description]
  }
}

resource "proxmox_virtual_environment_vm" "worker" {
  count = var.worker_count

  name      = local.worker_nodes[count.index].name
  node_name = var.node_name
  tags      = [var.environment, "opord", "worker"]

  clone {
    vm_id = var.template_vmid
    full  = true
  }

  agent {
    enabled = true
  }

  cpu {
    cores = var.worker_specs.cpu
    type  = "host"
  }

  memory {
    dedicated = var.worker_specs.memory
  }

  disk {
    datastore_id = var.datastore_id
    interface    = "scsi0"
    size         = var.worker_specs.disk
  }

  network_device {
    bridge = var.network_bridge
  }

  initialization {
    dns {
      domain  = var.dns_domain
      servers = var.dns_servers
    }
    ip_config {
      ipv4 {
        address = "${local.worker_nodes[count.index].ip}/${var.netmask_bits}"
        gateway = var.gateway
      }
    }
    user_account {
      username = var.ssh_user
      keys     = var.ssh_public_key != "" ? [var.ssh_public_key] : []
    }
  }

  lifecycle {
    ignore_changes = [description]
  }
}
