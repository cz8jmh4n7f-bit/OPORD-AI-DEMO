output "vnet_id" {
  value       = azurerm_virtual_network.this.id
  description = "VNet ARM resource ID."
}

output "vnet_name" {
  value       = azurerm_virtual_network.this.name
  description = "VNet name."
}

output "vnet_cidr" {
  value       = var.vnet_cidr
  description = "Allocated /22 CIDR (mirrored for audit/runbook)."
}

output "subnet_ids" {
  value       = [for s in azurerm_subnet.this : s.id]
  description = "Subnet resource IDs (3x /24)."
}

output "subnet_cidrs" {
  value       = local.subnet_cidrs
  description = "Carved subnet CIDRs."
}

output "nsg_id" {
  value       = azurerm_network_security_group.this.id
  description = "NSG resource ID."
}
