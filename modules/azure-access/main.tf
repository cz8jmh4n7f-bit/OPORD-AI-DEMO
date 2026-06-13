locals {
  group_display = "${var.group_prefix}${var.project_name}"
  # Scope: the whole subscription, or narrowed to a resource group when set.
  scope = var.resource_group == "" ? "/subscriptions/${var.subscription_id}" : "/subscriptions/${var.subscription_id}/resourceGroups/${var.resource_group}"
  has_members = length(var.user_principal_names) > 0
}

# Resolve the member UPNs/emails to Entra object IDs. Only query when members are
# supplied - azuread_users errors if every lookup list is empty. A typo'd or
# missing user fails the apply loudly (ignore_missing defaults false), so the
# operator knows access was NOT granted to that person.
data "azuread_users" "members" {
  count                = local.has_members ? 1 : 0
  user_principal_names = var.user_principal_names
}

# The project group. OPORD owns the full membership list, so a day-2
# add/remove member is just a re-apply with an updated user_principal_names.
resource "azuread_group" "this" {
  display_name     = local.group_display
  description      = "OPORD access-vending group for project ${var.project_name}"
  security_enabled = true
  members          = local.has_members ? data.azuread_users.members[0].object_ids : []
}

# Resolve the role definition ID at the scope - needed by the PIM-eligible
# resource (the permanent assignment can use the role name directly).
data "azurerm_role_definition" "this" {
  name  = var.role_name
  scope = local.scope
}

# Permanent binding: the group holds the role at the scope until removed.
# principal_type "Group" avoids the AAD principal-not-found check during the
# group's replication window.
resource "azurerm_role_assignment" "this" {
  count                = var.pim_eligible ? 0 : 1
  scope                = local.scope
  role_definition_name = var.role_name
  principal_id         = azuread_group.this.object_id
  principal_type       = "Group"
}

# PIM-eligible binding: the group is *eligible* for the role; members activate
# it just-in-time (duration governed by the tenant's PIM policy). Permanent
# eligibility (no expiration). Requires Microsoft Entra ID P2 on the tenant.
resource "azurerm_pim_eligible_role_assignment" "this" {
  count              = var.pim_eligible ? 1 : 0
  scope              = local.scope
  role_definition_id = data.azurerm_role_definition.this.id
  principal_id       = azuread_group.this.object_id

  schedule {
    expiration {
      duration_days = 365
    }
  }

  justification = "OPORD access-vending project ${var.project_name} (PIM-eligible)"
}
