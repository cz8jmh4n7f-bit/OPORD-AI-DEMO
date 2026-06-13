# Companion: Azure Policy. Mocked azurerm, plan-only. Asserts the 5 built-in
# assignments exist and the allowed-locations parameter flows through.

mock_provider "azurerm" {}

variables {
  subscription_id          = "00000000-0000-0000-0000-000000000000"
  subscription_resource_id = "/subscriptions/00000000-0000-0000-0000-000000000000"
  name_prefix              = "opord"
  csa_id                   = "acme"
  allowed_locations        = ["westeurope", "northeurope"]
}

run "five_assignments_present" {
  command = plan

  assert {
    condition     = length(output.assignment_names) == 5
    error_message = "expected 5 policy assignment names"
  }
  assert {
    condition     = azurerm_subscription_policy_assignment.allowed_locations.name == "opord-acme-allowed-locations"
    error_message = "allowed-locations assignment name wrong"
  }
}

run "allowed_locations_flow_through" {
  command = plan

  assert {
    condition     = length(output.allowed_locations) == 2
    error_message = "allowed_locations output should echo the two regions"
  }
}
