# L3 RBAC: mocked azurerm + azuread, plan-only. Asserts role-definition naming
# and the create_groups toggle (PIM-safe group path vs role-definitions-only).

mock_provider "azurerm" {}
mock_provider "azuread" {}

variables {
  subscription_id          = "00000000-0000-0000-0000-000000000000"
  subscription_resource_id = "/subscriptions/00000000-0000-0000-0000-000000000000"
  name_prefix              = "opord"
  csa_id                   = "acme"
}

run "three_role_definitions_named" {
  command = plan

  assert {
    condition     = length(azurerm_role_definition.this) == 3
    error_message = "default role_tier should yield 3 custom roles (admin/manager/custom1)"
  }
  assert {
    condition     = azurerm_role_definition.this["admin"].name == "opord-acme-admin"
    error_message = "admin role name wrong"
  }
}

run "groups_created_by_default" {
  command = plan

  assert {
    condition     = length(azuread_group.this) == 3
    error_message = "create_groups defaults true to 3 Entra groups"
  }
  assert {
    condition     = length(azurerm_role_assignment.group_to_role) == 3
    error_message = "3 group to role assignments expected"
  }
}

run "create_groups_false_skips_groups" {
  command = plan

  variables {
    create_groups = false
  }

  assert {
    condition     = length(azuread_group.this) == 0
    error_message = "create_groups=false must skip Entra groups"
  }
  assert {
    condition     = length(azurerm_role_assignment.group_to_role) == 0
    error_message = "no assignments when there are no groups"
  }
  assert {
    condition     = length(azurerm_role_definition.this) == 3
    error_message = "role definitions must still be created when groups are skipped"
  }
}
