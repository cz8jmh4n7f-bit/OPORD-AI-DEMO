# L2 baseline: mocked azurerm, plan-only. Asserts the 3 base RG names + the
# Defender Free-vs-Standard branch.

mock_provider "azurerm" {}

variables {
  subscription_id = "00000000-0000-0000-0000-000000000000"
  location        = "westeurope"
  name_prefix     = "opord"
  csa_id          = "acme"
  csa_cloud_name  = "dev"
}

run "base_rgs_named_correctly" {
  command = plan

  assert {
    condition     = azurerm_resource_group.network.name == "opord-acme-network-rg"
    error_message = "network RG name wrong"
  }
  assert {
    condition     = azurerm_resource_group.security.name == "opord-acme-security-rg"
    error_message = "security RG name wrong"
  }
  assert {
    condition     = azurerm_resource_group.logs.name == "opord-acme-logs-rg"
    error_message = "logs RG name wrong"
  }
  assert {
    condition     = output.defender_tier == "Free"
    error_message = "empty defender_plans_standard should report Free tier"
  }
}

run "free_tier_pricing_when_no_standard_plans" {
  command = plan

  assert {
    condition     = length(azurerm_security_center_subscription_pricing.free_baseline) == 1
    error_message = "Free-tier pricing resource should be present when no Standard plans"
  }
  assert {
    condition     = length(azurerm_security_center_subscription_pricing.standard) == 0
    error_message = "no Standard pricing resources when defender_plans_standard is empty"
  }
}

run "standard_plans_switch_tiers" {
  command = plan

  variables {
    defender_plans_standard = ["VirtualMachines", "KeyVaults"]
  }

  assert {
    condition     = length(azurerm_security_center_subscription_pricing.standard) == 2
    error_message = "two Standard pricing resources expected"
  }
  assert {
    condition     = length(azurerm_security_center_subscription_pricing.free_baseline) == 0
    error_message = "Free-tier baseline should be absent once Standard plans are set"
  }
  assert {
    condition     = output.defender_tier == "Mixed (Free + Standard)"
    error_message = "defender_tier should report Mixed"
  }
}
