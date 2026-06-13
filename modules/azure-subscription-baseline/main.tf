locals {
  base_tags = merge(var.tags, {
    Project   = var.name_prefix
    CsaId     = var.csa_id
    Cloud     = var.csa_cloud_name
    ManagedBy = "opord"
  })
  rg_names = {
    network  = "${var.name_prefix}-${var.csa_id}-network-rg"
    security = "${var.name_prefix}-${var.csa_id}-security-rg"
    logs     = "${var.name_prefix}-${var.csa_id}-logs-rg"
  }
}

# Resource Provider registration. Idempotent: re-running on an already-
# registered RP is a no-op. Some providers (Microsoft.Security) require a few
# minutes after first registration before Defender APIs return data - that's
# fine, L2 doesn't read those APIs.
resource "azurerm_resource_provider_registration" "this" {
  for_each = toset(var.resource_providers)
  name     = each.value
}

# Three foundational RGs. Every downstream layer creates resources inside one
# of these; centralising the create here means the RGs are the only objects
# L2 manages, which keeps L2 destroys clean and the dependency graph obvious.
resource "azurerm_resource_group" "network" {
  name     = local.rg_names.network
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_resource_group" "security" {
  name     = local.rg_names.security
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_resource_group" "logs" {
  name     = local.rg_names.logs
  location = var.location
  tags     = local.base_tags
}

# Defender for Cloud - Free tier always-on (CSPM = compliance scanning, $0/mo).
# Standard-tier plans are opt-in via defender_plans_standard.
resource "azurerm_security_center_subscription_pricing" "free_baseline" {
  count         = length(var.defender_plans_standard) == 0 ? 1 : 0
  tier          = "Free"
  resource_type = "VirtualMachines"
  depends_on    = [azurerm_resource_provider_registration.this]
}

resource "azurerm_security_center_subscription_pricing" "standard" {
  for_each      = toset(var.defender_plans_standard)
  tier          = "Standard"
  resource_type = each.value
  depends_on    = [azurerm_resource_provider_registration.this]
}

# These two are not policy assignments but a related guardrail surface:
# enabling auto-provisioning so newly-discovered VMs are monitored without an
# operator step. Kept separate from the Free/Standard pricing toggles so the
# operator can opt out by setting count=0 in a fork if needed.
