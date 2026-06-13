# --- Data sources ---
data "vsphere_datacenter" "dc" {
  name = var.vsphere_datacenter
}

data "vsphere_datastore" "ds" {
  name          = var.vsphere_datastore
  datacenter_id = data.vsphere_datacenter.dc.id
}

data "vsphere_compute_cluster" "cluster" {
  name          = var.vsphere_cluster
  datacenter_id = data.vsphere_datacenter.dc.id
}

data "vsphere_network" "net" {
  name          = var.vsphere_network
  datacenter_id = data.vsphere_datacenter.dc.id
}

data "vsphere_virtual_machine" "template" {
  name          = var.template_name
  datacenter_id = data.vsphere_datacenter.dc.id
}

# --- Compute names and static IPs ---
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

  network_adapter_type = length(data.vsphere_virtual_machine.template.network_interface_types) > 0 ? data.vsphere_virtual_machine.template.network_interface_types[0] : "vmxnet3"

  ansible_inventory = templatefile("${path.module}/templates/inventory.tftpl", {
    vms      = local.vms
    ssh_user = var.ssh_user
  })
}

# --- VMs ---
resource "vsphere_virtual_machine" "vm" {
  count = var.vm_count

  name             = local.vms[count.index].name
  resource_pool_id = data.vsphere_compute_cluster.cluster.resource_pool_id
  datastore_id     = data.vsphere_datastore.ds.id
  folder           = var.vsphere_folder_path

  num_cpus  = var.specs.cpu
  memory    = var.specs.memory
  guest_id  = data.vsphere_virtual_machine.template.guest_id
  scsi_type = data.vsphere_virtual_machine.template.scsi_type
  firmware  = var.firmware

  network_interface {
    network_id   = data.vsphere_network.net.id
    adapter_type = local.network_adapter_type
  }

  disk {
    label            = "disk0"
    size             = var.specs.disk
    thin_provisioned = true
  }

  dynamic "disk" {
    for_each = var.data_disks
    content {
      label            = format("data%d", disk.key)
      size             = disk.value
      unit_number      = disk.key + 1
      thin_provisioned = true
    }
  }

  clone {
    template_uuid = data.vsphere_virtual_machine.template.id

    customize {
      linux_options {
        host_name = local.vms[count.index].name
        domain    = var.dns_suffix
      }

      network_interface {
        ipv4_address = local.vms[count.index].ip
        ipv4_netmask = var.netmask_bits
      }

      ipv4_gateway    = var.gateway
      dns_server_list = var.dns_servers
      dns_suffix_list = [var.dns_suffix]
    }
  }

  lifecycle {
    ignore_changes = [annotation]
  }
}
