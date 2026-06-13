output "vm_names" {
  description = "Hostnames of the created VMs."
  value       = [for n in local.vms : n.name]
}

output "vm_ips" {
  description = "Static IPs assigned to the VMs."
  value       = [for n in local.vms : n.ip]
}

output "vm_moids" {
  description = "vSphere managed object IDs of the created VMs."
  value       = vsphere_virtual_machine.vm[*].moid
}

output "ansible_inventory" {
  description = "Rendered Ansible inventory (INI) for optional post-provision config."
  value       = local.ansible_inventory
}
