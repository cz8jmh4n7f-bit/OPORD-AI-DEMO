variable "location" {
  type        = string
  description = "Azure region."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix. Vault name has tight rules (3-24 chars, alphanumeric + hyphens, must start with letter); module trims."
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}

variable "sku_name" {
  type        = string
  description = "Vault SKU: standard or premium."
  default     = "standard"
  validation {
    condition     = contains(["standard", "premium"], var.sku_name)
    error_message = "sku_name must be standard or premium."
  }
}

variable "purge_protection_enabled" {
  type        = bool
  description = "Block immediate purge - destroyed vaults sit in soft-delete for `soft_delete_retention_days` before hard delete. Required for prod; false for dev."
  default     = false
}

variable "soft_delete_retention_days" {
  type        = number
  description = "Days to retain soft-deleted vaults (7-90)."
  default     = 7
  validation {
    condition     = var.soft_delete_retention_days >= 7 && var.soft_delete_retention_days <= 90
    error_message = "soft_delete_retention_days must be between 7 and 90."
  }
}

variable "rbac_authorization_enabled" {
  type        = bool
  description = "Use Azure RBAC (recommended) instead of access policies."
  default     = true
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
