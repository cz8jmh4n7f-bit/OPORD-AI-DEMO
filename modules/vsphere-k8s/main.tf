# --- Data sources: resolve named vSphere objects to managed-object IDs ---
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

# --- Compute node names and static IPs ---
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

  network_adapter_type = length(data.vsphere_virtual_machine.template.network_interface_types) > 0 ? data.vsphere_virtual_machine.template.network_interface_types[0] : "vmxnet3"

  ansible_inventory = templatefile("${path.module}/templates/inventory.tftpl", {
    control_plane = local.control_plane_nodes
    workers       = local.worker_nodes
    ssh_user      = var.ssh_user
    cp_host       = var.control_plane_endpoint
    cp_port       = var.control_plane_endpoint_port
  })
}

# --- Control-plane VMs ---
resource "vsphere_virtual_machine" "control_plane" {
  count = var.control_plane_count

  name             = local.control_plane_nodes[count.index].name
  resource_pool_id = data.vsphere_compute_cluster.cluster.resource_pool_id
  datastore_id     = data.vsphere_datastore.ds.id
  folder           = var.vsphere_folder_path

  num_cpus = var.control_plane_specs.cpu
  memory   = var.control_plane_specs.memory
  guest_id = data.vsphere_virtual_machine.template.guest_id
  scsi_type = data.vsphere_virtual_machine.template.scsi_type
  firmware = var.firmware

  network_interface {
    network_id   = data.vsphere_network.net.id
    adapter_type = local.network_adapter_type
  }

  disk {
    label            = "disk0"
    size             = var.control_plane_specs.disk
    thin_provisioned = true
  }

  clone {
    template_uuid = data.vsphere_virtual_machine.template.id

    customize {
      linux_options {
        host_name = local.control_plane_nodes[count.index].name
        domain    = var.dns_suffix
      }

      network_interface {
        ipv4_address = local.control_plane_nodes[count.index].ip
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

# --- Worker VMs ---
resource "vsphere_virtual_machine" "worker" {
  count = var.worker_count

  name             = local.worker_nodes[count.index].name
  resource_pool_id = data.vsphere_compute_cluster.cluster.resource_pool_id
  datastore_id     = data.vsphere_datastore.ds.id
  folder           = var.vsphere_folder_path

  num_cpus = var.worker_specs.cpu
  memory   = var.worker_specs.memory
  guest_id = data.vsphere_virtual_machine.template.guest_id
  scsi_type = data.vsphere_virtual_machine.template.scsi_type
  firmware = var.firmware

  network_interface {
    network_id   = data.vsphere_network.net.id
    adapter_type = local.network_adapter_type
  }

  disk {
    label            = "disk0"
    size             = var.worker_specs.disk
    thin_provisioned = true
  }

  dynamic "disk" {
    for_each = var.worker_data_disks
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
        host_name = local.worker_nodes[count.index].name
        domain    = var.dns_suffix
      }

      network_interface {
        ipv4_address = local.worker_nodes[count.index].ip
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
