locals {
  base_tags = merge(var.tags, {
    Project   = var.name_prefix
    CsaId     = var.csa_id
    Cloud     = var.csa_cloud_name
    ManagedBy = "opord"
  })
  derived_alias = var.alias != "" ? var.alias : "${var.name_prefix}-${var.csa_id}-${var.csa_cloud_name}"
  derived_name  = var.subscription_name != "" ? var.subscription_name : "${var.name_prefix}-${var.csa_id}-${var.csa_cloud_name}"
}

# ---------------------------------------------------------------------------
# adopt mode: data lookup, no creation. The operator has manually granted the
# provisioning SP Owner on this subscription before running OPORD.
# ---------------------------------------------------------------------------
data "azurerm_subscription" "adopted" {
  count           = var.mode == "adopt" ? 1 : 0
  subscription_id = var.subscription_id
}

# Hard preconditions - fail fast with a clear message instead of an opaque
# downstream error if the operator passed an incompatible combo.
resource "null_resource" "preconditions" {
  triggers = {
    mode = var.mode
  }
  lifecycle {
    precondition {
      condition     = var.mode != "adopt" || var.subscription_id != ""
      error_message = "mode=adopt requires subscription_id."
    }
    precondition {
      condition     = var.mode != "create" || var.billing_scope_id != ""
      error_message = "mode=create requires billing_scope_id (MCA invoice section URI)."
    }
  }
}

# ---------------------------------------------------------------------------
# create mode: MCA subscription via alias resource. The SP needs Invoice
# Section Owner (or higher) on billing_scope_id. The alias name is
# tenant-unique; downstream IDs become available once the async create
# operation reports succeeded.
# ---------------------------------------------------------------------------
resource "azurerm_subscription" "created" {
  count             = var.mode == "create" ? 1 : 0
  alias             = local.derived_alias
  subscription_name = local.derived_name
  billing_scope_id  = var.billing_scope_id
  workload          = var.workload
  tags              = local.base_tags
}

# MCA propagation: even after CreateSubscription succeeds, role assignments
# made too quickly on the new subscription's scope can return 404 for a few
# minutes. Sleep here so L2 doesn't have to retry.
resource "time_sleep" "wait_for_propagation" {
  count           = var.mode == "create" ? 1 : 0
  depends_on      = [azurerm_subscription.created]
  create_duration = "${var.wait_seconds_after_create}s"
}
