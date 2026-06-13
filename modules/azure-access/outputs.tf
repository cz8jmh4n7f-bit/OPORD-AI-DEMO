output "group_id" {
  description = "Object ID of the managed Entra group."
  value       = azuread_group.this.object_id
}

output "group_name" {
  description = "Display name of the managed Entra group."
  value       = azuread_group.this.display_name
}

output "role_assignment_id" {
  description = "ID of the role assignment (permanent) or eligible assignment (PIM) binding the group to the role."
  value       = var.pim_eligible ? azurerm_pim_eligible_role_assignment.this[0].id : azurerm_role_assignment.this[0].id
}

output "role_name" {
  description = "The Azure RBAC role granted to the group."
  value       = var.role_name
}

output "pim_eligible" {
  description = "Whether the role is PIM-eligible (just-in-time) rather than a permanent assignment."
  value       = var.pim_eligible
}

output "scope" {
  description = "The scope (subscription or resource group) the role is granted at."
  value       = local.scope
}

output "member_count" {
  description = "Number of resolved members in the group."
  value       = local.has_members ? length(data.azuread_users.members[0].object_ids) : 0
}
