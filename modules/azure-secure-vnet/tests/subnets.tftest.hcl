# OpenTofu native test for the L4 secure-vnet module. Mocked azurerm provider,
# plan-only - asserts the /22 is carved into three /24 subnets and that the
# create_flow_logs toggle gates the Flow Log + Network Watcher data source.

# azurerm validates resource-ID formats even under mocks, so give the computed
# ids a real ARM shape (the random default would fail the subnet-ID parser in
# the NSG-association resource).
mock_provider "azurerm" {
  mock_resource "azurerm_virtual_network" {
    defaults = {
      id = "/subscriptions/x/resourceGroups/y/providers/Microsoft.Network/virtualNetworks/v"
    }
  }
  mock_resource "azurerm_subnet" {
    defaults = {
      id = "/subscriptions/x/resourceGroups/y/providers/Microsoft.Network/virtualNetworks/v/subnets/s"
    }
  }
  mock_resource "azurerm_network_security_group" {
    defaults = {
      id = "/subscriptions/x/resourceGroups/y/providers/Microsoft.Network/networkSecurityGroups/n"
    }
  }
}

variables {
  subscription_id              = "00000000-0000-0000-0000-000000000000"
  network_rg_name              = "opord-acme-network-rg"
  location                     = "westeurope"
  name_prefix                  = "opord"
  csa_id                       = "acme"
  vnet_cidr                    = "10.20.0.0/22"
  flow_logs_storage_account_id = "/subscriptions/x/resourceGroups/y/providers/Microsoft.Storage/storageAccounts/z"
}

run "three_24_subnets_from_22" {
  command = plan

  assert {
    condition     = length(azurerm_subnet.this) == 3
    error_message = "a /22 must yield exactly three /24 subnets"
  }
  assert {
    condition     = azurerm_subnet.this[0].address_prefixes[0] == "10.20.0.0/24"
    error_message = "first subnet should be 10.20.0.0/24"
  }
  assert {
    condition     = azurerm_subnet.this[2].address_prefixes[0] == "10.20.2.0/24"
    error_message = "third subnet should be 10.20.2.0/24"
  }
}

run "flow_logs_on_by_default" {
  command = plan

  assert {
    condition     = length(azurerm_network_watcher_flow_log.this) == 1
    error_message = "Flow Logs should be created by default"
  }
}

run "flow_logs_can_be_disabled" {
  command = plan

  variables {
    create_flow_logs = false
  }

  assert {
    condition     = length(azurerm_network_watcher_flow_log.this) == 0
    error_message = "create_flow_logs=false must skip the Flow Log"
  }
  assert {
    condition     = length(data.azurerm_network_watcher.this) == 0
    error_message = "create_flow_logs=false must skip the Network Watcher data source too"
  }
}
