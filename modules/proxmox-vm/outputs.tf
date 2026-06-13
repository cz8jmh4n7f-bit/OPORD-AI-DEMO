output "vm_names" {
  description = "Hostnames of the created VMs."
  value       = [for n in local.vms : n.name]
}

output "vm_ips" {
  description = "Static IPs assigned to the VMs."
  value       = [for n in local.vms : n.ip]
}

output "vm_ids" {
  description = "Proxmox VMIDs of the created VMs."
  value       = proxmox_virtual_environment_vm.vm[*].vm_id
}
