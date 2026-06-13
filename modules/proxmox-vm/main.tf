locals {
  ip_parts  = split(".", var.ip_start)
  ip_prefix = join(".", slice(local.ip_parts, 0, 3))
  ip_last   = tonumber(element(local.ip_parts, 3))

  vms = [
    for i in range(var.vm_count) : {
      name = format("%s-%02d", var.name_prefix, i + 1)
      ip   = format("%s.%d", local.ip_prefix, local.ip_last + i)
    }
  ]
}

resource "proxmox_virtual_environment_vm" "vm" {
  count = var.vm_count

  name      = local.vms[count.index].name
  node_name = var.node_name
  tags      = [var.environment, "opord"]

  clone {
    vm_id = var.template_vmid
    full  = true
  }

  agent {
    enabled = true
  }

  cpu {
    cores = var.cores
    type  = "host"
  }

  memory {
    dedicated = var.memory_mb
  }

  disk {
    datastore_id = var.datastore_id
    interface    = "scsi0"
    size         = var.disk_gb
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
        address = "${local.vms[count.index].ip}/${var.netmask_bits}"
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
