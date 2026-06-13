output "assignment_names" {
  value = [
    azurerm_subscription_policy_assignment.allowed_locations.name,
    azurerm_subscription_policy_assignment.audit_unmanaged_disks.name,
    azurerm_subscription_policy_assignment.kv_purge_protection.name,
    azurerm_subscription_policy_assignment.storage_restrict_network.name,
    azurerm_subscription_policy_assignment.kv_diagnostic_logs.name,
  ]
  description = "List of policy assignment resource names. Surfaced for audit."
}

output "allowed_locations" {
  value       = var.allowed_locations
  description = "Allowed-locations list applied. Mirrored for runbook/audit."
}
