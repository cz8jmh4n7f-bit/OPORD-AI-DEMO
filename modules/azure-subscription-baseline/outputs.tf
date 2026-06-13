output "network_rg_name" {
  value       = azurerm_resource_group.network.name
  description = "Network resource group (L4 secure-vnet creates VNet/NSG/Flow Logs inside)."
}

output "security_rg_name" {
  value       = azurerm_resource_group.security.name
  description = "Security resource group (Key Vault baseline + Defender resources)."
}

output "logs_rg_name" {
  value       = azurerm_resource_group.logs.name
  description = "Logs resource group (L5 security-hardening creates Log Analytics + Storage CMK inside)."
}

output "defender_tier" {
  value       = length(var.defender_plans_standard) == 0 ? "Free" : "Mixed (Free + Standard)"
  description = "Effective Defender tier the operator chose. Surfaced for audit/runbook."
}
