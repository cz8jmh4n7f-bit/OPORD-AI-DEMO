# Access-vending module: Entra group + Azure RBAC. Mocked azurerm + azuread,
# plan-only. Asserts the group naming, the scope (subscription vs resource
# group), and - importantly - the pim_eligible toggle switching between a
# permanent azurerm_role_assignment and an azurerm_pim_eligible_role_assignment.
# This covers the PIM happy-path config offline (a live PIM apply needs Entra P2).

mock_provider "azurerm" {
  mock_data "azurerm_role_definition" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/providers/Microsoft.Authorization/roleDefinitions/11111111-1111-1111-1111-111111111111"
    }
  }
}
mock_provider "azuread" {
  mock_resource "azuread_group" {
    defaults = {
      object_id = "22222222-2222-2222-2222-222222222222"
    }
  }
}

variables {
  subscription_id = "00000000-0000-0000-0000-000000000000"
  project_name    = "alpha"
  role_name       = "Reader"
}

run "permanent_assignment_by_default" {
  command = plan

  assert {
    condition     = length(azurerm_role_assignment.this) == 1
    error_message = "a permanent role assignment is expected when pim_eligible is false (default)"
  }
  assert {
    condition     = length(azurerm_pim_eligible_role_assignment.this) == 0
    error_message = "no PIM-eligible assignment when pim_eligible is false"
  }
  assert {
    condition     = azuread_group.this.display_name == "opord-alpha"
    error_message = "group display name should be <group_prefix><project_name>"
  }
  assert {
    condition     = azurerm_role_assignment.this[0].scope == "/subscriptions/00000000-0000-0000-0000-000000000000"
    error_message = "default scope should be the whole subscription"
  }
}

run "pim_eligible_when_enabled" {
  command = plan

  variables {
    pim_eligible = true
  }

  assert {
    condition     = length(azurerm_pim_eligible_role_assignment.this) == 1
    error_message = "a PIM-eligible assignment is expected when pim_eligible is true"
  }
  assert {
    condition     = length(azurerm_role_assignment.this) == 0
    error_message = "no permanent assignment when pim_eligible is true"
  }
}

run "scope_narrows_to_resource_group" {
  command = plan

  variables {
    resource_group = "myrg"
  }

  assert {
    condition     = azurerm_role_assignment.this[0].scope == "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myrg"
    error_message = "scope should narrow to the resource group when resource_group is set"
  }
}
