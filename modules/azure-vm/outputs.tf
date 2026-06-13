output "resource_group_name" {
  description = "Name of the resource group holding the VM(s) and supporting resources."
  value       = azurerm_resource_group.this.name
}

output "vm_ids" {
  description = "Resource IDs of the created VMs."
  value       = azurerm_linux_virtual_machine.this[*].id
}

output "vm_names" {
  description = "VM names."
  value       = azurerm_linux_virtual_machine.this[*].name
}

output "public_ips" {
  description = "Public IPv4 addresses (empty when associate_public_ip=false)."
  value       = var.associate_public_ip ? azurerm_public_ip.this[*].ip_address : []
}

output "private_ips" {
  description = "Private IPv4 addresses inside the VNet."
  value       = azurerm_network_interface.this[*].private_ip_address
}
