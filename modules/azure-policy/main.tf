locals {
  base_tags = merge(var.tags, {
    Project   = var.name_prefix
    CsaId     = var.csa_id
    ManagedBy = "opord"
  })

  # Built-in policy definition IDs (Microsoft-managed; stable GUIDs).
  builtin = {
    allowed_locations         = "/providers/Microsoft.Authorization/policyDefinitions/e56962a6-4747-49cd-b67b-bf8b01975c4c"
    audit_unmanaged_disks     = "/providers/Microsoft.Authorization/policyDefinitions/06a78e20-9358-41c9-923c-fb736d382a4d"
    kv_purge_protection       = "/providers/Microsoft.Authorization/policyDefinitions/0b60c0b2-2dc2-4e1c-b5c9-abbed971de53"
    storage_restrict_network  = "/providers/Microsoft.Authorization/policyDefinitions/34c877ad-507e-4c82-993e-3452a6e0ad3c"
    kv_diagnostic_logs        = "/providers/Microsoft.Authorization/policyDefinitions/cf820ca0-f99e-4f3e-84fb-66e913812d21"
  }
}

resource "azurerm_subscription_policy_assignment" "allowed_locations" {
  name                 = "${var.name_prefix}-${var.csa_id}-allowed-locations"
  display_name         = "Allowed Locations (${var.csa_id})"
  description          = "Resources must be in ${join(", ", var.allowed_locations)}."
  policy_definition_id = local.builtin.allowed_locations
  subscription_id      = var.subscription_resource_id

  parameters = jsonencode({
    listOfAllowedLocations = {
      value = var.allowed_locations
    }
  })
}

resource "azurerm_subscription_policy_assignment" "audit_unmanaged_disks" {
  name                 = "${var.name_prefix}-${var.csa_id}-audit-unmanaged-disks"
  display_name         = "Audit non-managed disks (${var.csa_id})"
  policy_definition_id = local.builtin.audit_unmanaged_disks
  subscription_id      = var.subscription_resource_id
}

resource "azurerm_subscription_policy_assignment" "kv_purge_protection" {
  name                 = "${var.name_prefix}-${var.csa_id}-kv-purge-protection"
  display_name         = "Key Vaults must have purge protection (${var.csa_id})"
  policy_definition_id = local.builtin.kv_purge_protection
  subscription_id      = var.subscription_resource_id
}

resource "azurerm_subscription_policy_assignment" "storage_restrict_network" {
  name                 = "${var.name_prefix}-${var.csa_id}-storage-restrict-network"
  display_name         = "Storage accounts should restrict network access (${var.csa_id})"
  policy_definition_id = local.builtin.storage_restrict_network
  subscription_id      = var.subscription_resource_id
}

resource "azurerm_subscription_policy_assignment" "kv_diagnostic_logs" {
  name                 = "${var.name_prefix}-${var.csa_id}-kv-diagnostic-logs"
  display_name         = "Key Vaults should send diagnostic logs (${var.csa_id})"
  policy_definition_id = local.builtin.kv_diagnostic_logs
  subscription_id      = var.subscription_resource_id
}
