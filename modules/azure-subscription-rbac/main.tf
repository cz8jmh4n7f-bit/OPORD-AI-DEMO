locals {
  base_tags = merge(var.tags, {
    Project   = var.name_prefix
    CsaId     = var.csa_id
    ManagedBy = "opord"
  })
}

# Custom role definitions, scoped to this subscription only (least privilege
# vs tenant scope). Naming: <prefix>-<csa>-<tier> e.g. opord-acme-admin.
resource "azurerm_role_definition" "this" {
  for_each    = var.role_tier
  name        = "${var.name_prefix}-${var.csa_id}-${each.key}"
  scope       = var.subscription_resource_id
  description = "OPORD managed role (tier=${each.key}) for ${var.csa_id}"

  permissions {
    actions          = each.value.actions
    not_actions      = each.value.not_actions
    data_actions     = each.value.data_actions
    not_data_actions = each.value.not_data_actions
  }

  assignable_scopes = [var.subscription_resource_id]
}

# One Entra group per tier. Membership is managed by OPORD's Graph client
# (internal/azure) - operators add/remove users via `opord account members`
# (mirrors `opord project members` for the AWS Identity Center primitive).
# Group names mirror role names for operator clarity.
#
# Creating Entra groups needs a DIRECTORY-plane permission (Graph
# Group.ReadWrite.All app permission, OR the SP holding the "Groups
# Administrator" directory role) - distinct from the Owner ARM role that the
# role-definition + assignment steps need. When the SP lacks it, set
# create_groups=false: the layer still creates the custom role DEFINITIONS
# (the hard part), and the operator binds them to existing groups/users
# out-of-band. Defaulting to true keeps the PIM-safe group-based model
# (ADR-0009) for tenants where the SP has the directory permission.
resource "azuread_group" "this" {
  for_each         = var.create_groups ? var.role_tier : {}
  display_name     = "${var.name_prefix}-${var.csa_id}-${each.key}"
  description      = "OPORD managed access group for ${var.csa_id} (tier=${each.key})"
  security_enabled = true
}

# Assign each custom role to its sibling group. This is the PIM-safe binding:
# direct user assignments would collide with PIM eligibility roles, group
# assignments don't. Skipped when create_groups=false (no group to bind to).
resource "azurerm_role_assignment" "group_to_role" {
  for_each           = var.create_groups ? var.role_tier : {}
  scope              = var.subscription_resource_id
  role_definition_id = azurerm_role_definition.this[each.key].role_definition_resource_id
  principal_id       = azuread_group.this[each.key].object_id
}
