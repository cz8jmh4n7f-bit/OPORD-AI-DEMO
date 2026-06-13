variable "region" {
  type        = string
  description = "AWS region for the provider."
}

variable "assume_role_arn" {
  type        = string
  description = "Role ARN in the member account to assume."
}

variable "name" {
  type        = string
  description = "Name prefix for roles (e.g. opord-<csa_id>)."
}

variable "saml_metadata_document" {
  type        = string
  description = "Azure AD / Entra ID federation metadata XML (the IdP metadata)."
  sensitive   = true
}

variable "session_duration" {
  type        = number
  description = "Max federated session duration (seconds, 3600-43200)."
  default     = 28800

  validation {
    condition     = var.session_duration >= 3600 && var.session_duration <= 43200
    error_message = "session_duration must be between 3600 and 43200 seconds."
  }
}

variable "roles" {
  type        = map(list(string))
  description = "Role name suffix to list of managed policy ARNs to attach. Default: Admin / Manager / ReadOnly."
  default = {
    Admin    = ["arn:aws:iam::aws:policy/AdministratorAccess"]
    Manager  = ["arn:aws:iam::aws:policy/PowerUserAccess"]
    ReadOnly = ["arn:aws:iam::aws:policy/ReadOnlyAccess"]
  }
}

variable "tags" {
  type        = map(string)
  description = "Tags applied to all resources."
  default     = {}
}
