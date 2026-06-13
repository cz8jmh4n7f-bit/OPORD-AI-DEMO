variable "subscription_id" {
  type        = string
  description = "Target subscription whose scope the role is granted at."
}

variable "resource_group" {
  type        = string
  description = "When set, narrows the scope from the whole subscription to this resource group."
  default     = ""
}

variable "project_name" {
  type        = string
  description = "Project name; the Entra group is named <group_prefix><project_name>."
}

variable "group_prefix" {
  type        = string
  description = "Prefix for the managed Entra group display name."
  default     = "opord-"
}

variable "role_name" {
  type        = string
  description = "Built-in (or custom) Azure RBAC role assigned to the group, e.g. Reader, Contributor."
  default     = "Reader"
}

variable "user_principal_names" {
  type        = list(string)
  description = "Existing Entra users (UPN or email) added to the project group. May be empty."
  default     = []
}

variable "pim_eligible" {
  type        = bool
  description = "When true, make the group ELIGIBLE for the role via PIM (just-in-time activation) instead of a permanent assignment. Requires Microsoft Entra ID P2."
  default     = false
}

variable "tags" {
  type        = map(string)
  description = "Tags applied where supported (Entra groups don't carry Azure tags; reserved for parity)."
  default     = {}
}
