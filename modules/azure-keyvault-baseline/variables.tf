variable "subscription_id" {
  type        = string
  description = "Target subscription GUID (L1 output)."
}

variable "security_rg_name" {
  type        = string
  description = "Security resource group name (L2 output) where the vault lives."
}

variable "location" {
  type        = string
  description = "Azure region."
  default     = "westeurope"
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

variable "sku_name" {
  type        = string
  description = "Vault SKU: standard or premium (premium = HSM-backed)."
  default     = "standard"
  validation {
    condition     = contains(["standard", "premium"], var.sku_name)
    error_message = "sku_name must be standard or premium."
  }
}

variable "soft_delete_retention_days" {
  type        = number
  description = "Days a deleted vault stays recoverable (7-90)."
  default     = 90
  validation {
    condition     = var.soft_delete_retention_days >= 7 && var.soft_delete_retention_days <= 90
    error_message = "soft_delete_retention_days must be between 7 and 90."
  }
}

variable "create_cmk" {
  type        = bool
  description = <<-EOT
    Create a Customer-Managed Key inside the vault for L5 to use as the
    Storage Account encryption key. DEFAULTS FALSE for V1: creating the
    role assignment that lets the SP write to the vault data plane needs
    Microsoft.Authorization/roleAssignments/write, which is part of Owner
    or User Access Administrator. If your SP is Owner-on-subscription but
    not on the KV scope (common), this 403s. With create_cmk=false, L5
    falls back to Microsoft-managed encryption (still encrypted at rest).
    Opt in once your SP has the right role.
  EOT
  default     = false
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
