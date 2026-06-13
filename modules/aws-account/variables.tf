variable "region" {
  type        = string
  description = "Provider region (Organizations is global; region still required)."
  default     = "us-east-1"
}

variable "account_name" {
  type        = string
  description = "Member account name (e.g. opord-<csa_id>-<env>)."
}

variable "email" {
  type        = string
  description = "Unique root email for the account (use + aliases or catch-all)."

  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.email))
    error_message = "email must be a valid address."
  }
}

variable "ou_id" {
  type        = string
  description = "Parent OU ID to place the account in. Empty = stays under root."
  default     = ""
}

variable "access_role_name" {
  type        = string
  description = "Cross-account bootstrap role Organizations creates in the new account."
  default     = "OrganizationAccountAccessRole"
}

variable "close_on_deletion" {
  type        = bool
  description = "If true, `tofu destroy` CLOSES the account (irreversible, 90-day window). Default false - OPORD removes from the OU/forgets instead, and closure is a guarded day-2 action."
  default     = false
}

variable "tags" {
  type        = map(string)
  description = "Account tags (csa_id, owner, environment, project, ...)."
  default     = {}
}
