# OpenTofu native test for the L1 subscription module. Plan-only with a MOCKED
# azurerm provider so it needs no Azure credentials and contacts nothing -
# `tofu test` asserts the module's naming + output wiring + preconditions.
# Requires OpenTofu 1.7+ (mock_provider).

mock_provider "azurerm" {}
mock_provider "time" {}

run "adopt_mode_builds_resource_id" {
  command = plan

  variables {
    mode            = "adopt"
    subscription_id = "00000000-0000-0000-0000-000000000000"
    name_prefix     = "opord"
    csa_id          = "acme"
    csa_cloud_name  = "dev"
  }

  assert {
    condition     = output.mode == "adopt"
    error_message = "mode output should echo adopt"
  }

  assert {
    condition     = output.subscription_resource_id == "/subscriptions/00000000-0000-0000-0000-000000000000"
    error_message = "adopt mode must build the subscription resource id from the supplied GUID"
  }
}

run "adopt_without_subscription_id_fails" {
  command = plan

  variables {
    mode           = "adopt"
    name_prefix    = "opord"
    csa_id         = "acme"
    csa_cloud_name = "dev"
    # subscription_id deliberately omitted
  }

  expect_failures = [
    null_resource.preconditions,
  ]
}

run "create_mode_builds_alias" {
  command = plan

  variables {
    mode              = "create"
    billing_scope_id  = "/providers/Microsoft.Billing/billingAccounts/x/billingProfiles/y/invoiceSections/z"
    subscription_name = "opord-acme-dev"
    name_prefix       = "opord"
    csa_id            = "acme"
    csa_cloud_name    = "dev"
  }

  assert {
    condition     = output.mode == "create"
    error_message = "mode output should echo create"
  }
}
