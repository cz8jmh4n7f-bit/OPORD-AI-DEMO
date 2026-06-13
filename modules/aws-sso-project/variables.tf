variable "region" {
  type        = string
  description = "AWS region for the provider. Identity Center is global, but the provider still needs a region."
  default     = "us-east-1"
}

variable "project_name" {
  type        = string
  description = "Project name. Becomes the Identity Center group display name (prefixed opord-)."
}

variable "account_id" {
  type        = string
  description = "Existing AWS account ID (12 digits) the project's group is granted access to."

  validation {
    condition     = can(regex("^[0-9]{12}$", var.account_id))
    error_message = "account_id must be a 12-digit AWS account ID."
  }
}

variable "user_names" {
  type        = list(string)
  description = "Existing Identity Center usernames to add to the project group. Add more later and re-apply."
  default     = []
}

# --- Permission set: create a new one, OR reference an existing one by ARN ---

variable "permission_set_name" {
  type        = string
  description = "If set, OPORD creates a permission set with this name and attaches managed_policy_arns. Leave empty to reference existing_permission_set_arn instead."
  default     = ""
}

variable "managed_policy_arns" {
  type        = list(string)
  description = "AWS managed policy ARNs to attach to the created permission set (only when permission_set_name is set)."
  default     = ["arn:aws:iam::aws:policy/ReadOnlyAccess"]
}

variable "session_duration" {
  type        = string
  description = "Permission set session duration (ISO-8601), e.g. PT8H. Only for a created permission set."
  default     = "PT8H"
}

variable "existing_permission_set_arn" {
  type        = string
  description = "ARN of an existing permission set to use instead of creating one. Used only when permission_set_name is empty."
  default     = ""
}

# --- Identity Center instance: auto-derived if not supplied ---

variable "sso_instance_arn" {
  type        = string
  description = "Identity Center instance ARN. Empty = use the org's only instance (data source)."
  default     = ""
}

variable "identity_store_id" {
  type        = string
  description = "Identity Center identity store ID. Empty = derive from the instance (data source)."
  default     = ""
}

variable "group_prefix" {
  type        = string
  description = "Prefix for the managed group display name."
  default     = "opord-"
}
