variable "subscription_id" {
  type        = string
  description = "Target subscription GUID (L1 output)."
}

variable "subscription_resource_id" {
  type        = string
  description = "Full ARM resource ID of the subscription (L1 output). Used as scope for role definitions + assignments."
}

variable "name_prefix" {
  type        = string
  description = "OPORD naming prefix."
  default     = "opord"
}

variable "csa_id" {
  type        = string
  description = "Customer/project identifier."
}

variable "role_tier" {
  type = map(object({
    actions          = list(string)
    not_actions      = list(string)
    data_actions     = list(string)
    not_data_actions = list(string)
  }))
  description = <<-EOT
    Custom role definitions keyed by tier name. The module creates one role
    per key (admin / manager / custom1 by default) and one Entra group with
    the same suffix. Empty maps for fields you don't need.
  EOT
  default = {
    admin = {
      actions          = ["*"]
      not_actions      = ["Microsoft.Authorization/elevateAccess/Action"]
      data_actions     = []
      not_data_actions = []
    }
    manager = {
      actions = [
        "Microsoft.Resources/subscriptions/resourceGroups/read",
        "Microsoft.Resources/subscriptions/resourceGroups/write",
        "Microsoft.Compute/*",
        "Microsoft.Network/*",
        "Microsoft.Storage/*",
      ]
      not_actions      = []
      data_actions     = []
      not_data_actions = []
    }
    custom1 = {
      actions = [
        "Microsoft.Resources/subscriptions/resourceGroups/read",
        "Microsoft.Compute/virtualMachines/read",
        "Microsoft.Compute/virtualMachines/start/action",
        "Microsoft.Compute/virtualMachines/restart/action",
      ]
      not_actions      = []
      data_actions     = []
      not_data_actions = []
    }
  }
}

variable "create_groups" {
  type        = bool
  description = <<-EOT
    Create one Entra group per role tier and bind the custom role to it
    (PIM-safe, ADR-0009). Needs a directory-plane permission the SP may not
    have (Graph Group.ReadWrite.All, or the "Groups Administrator" directory
    role) - distinct from the Owner ARM role. When false, only the custom
    role DEFINITIONS are created; the operator binds them to existing
    identities out-of-band. Default true.
  EOT
  default     = true
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
