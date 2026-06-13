output "role_definition_ids" {
  value       = { for k, r in azurerm_role_definition.this : k => r.role_definition_resource_id }
  description = "Map of tier name to custom role definition resource ID."
}

output "group_ids" {
  value       = { for k, g in azuread_group.this : k => g.object_id }
  description = "Map of tier name to Entra group object ID. Operators add members to these via the OPORD Graph helper."
}

output "group_display_names" {
  value       = { for k, g in azuread_group.this : k => g.display_name }
  description = "Map of tier name to human-readable group name. Surfaced in CLI/audit."
}
