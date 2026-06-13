variable "mode" {
  type        = string
  description = "adopt: use an existing subscription_id. create: provision a new subscription via MCA billing scope."
  default     = "adopt"
  validation {
    condition     = contains(["adopt", "create"], var.mode)
    error_message = "mode must be 'adopt' or 'create'."
  }
}

variable "subscription_id" {
  type        = string
  description = "Required when mode=adopt. The GUID of the existing subscription OPORD is taking over."
  default     = ""
}

variable "subscription_name" {
  type        = string
  description = "Required when mode=create. Friendly name for the new subscription."
  default     = ""
}

variable "billing_scope_id" {
  type        = string
  description = <<-EOT
    Required when mode=create. MCA invoice-section URI in the form
    /providers/Microsoft.Billing/billingAccounts/{ba}/billingProfiles/{bp}/invoiceSections/{is}
    The provisioning service principal needs Invoice Section Owner (or higher)
    on this scope.
  EOT
  default     = ""
}

variable "workload" {
  type        = string
  description = "MCA workload: Production or DevTest."
  default     = "Production"
  validation {
    condition     = contains(["Production", "DevTest"], var.workload)
    error_message = "workload must be Production or DevTest."
  }
}

variable "alias" {
  type        = string
  description = "Subscription alias (must be unique within the tenant). Used as the resource name; defaults to name_prefix-csa_id."
  default     = ""
}

variable "name_prefix" {
  type        = string
  description = "OPORD naming prefix (e.g. opord)."
  default     = "opord"
}

variable "csa_id" {
  type        = string
  description = "Customer/project identifier (e.g. acme-prod). Used in subscription name + tags + alias derivation."
}

variable "csa_cloud_name" {
  type        = string
  description = "Environment classifier appended to the subscription name (e.g. prod, stage, dev)."
  default     = "dev"
}

variable "wait_seconds_after_create" {
  type        = number
  description = "Seconds to sleep after CreateSubscription returns, so eventual-consistency settles before L2 starts. MCA propagation is typically 60-300s."
  default     = 120
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource. Used as subscription tags when mode=create (read-only when mode=adopt)."
  default     = {}
}
