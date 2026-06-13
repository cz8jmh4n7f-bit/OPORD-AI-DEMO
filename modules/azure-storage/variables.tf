variable "location" {
  type        = string
  description = "Azure region."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix; storage account name has tight rules (lowercase + digits, <= 24 chars). Module hashes/truncates as needed."
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}

variable "account_tier" {
  type        = string
  description = "Storage Account tier: Standard or Premium."
  default     = "Standard"
  validation {
    condition     = contains(["Standard", "Premium"], var.account_tier)
    error_message = "account_tier must be Standard or Premium."
  }
}

variable "replication_type" {
  type        = string
  description = "Replication: LRS / GRS / ZRS / RAGRS / GZRS / RAGZRS."
  default     = "LRS"
}

variable "containers" {
  type        = list(string)
  description = "Blob container names to create. Empty = no containers (account only)."
  default     = []
}

variable "allow_blob_public_access" {
  type        = bool
  description = "Allow anonymous blob access at the account level. Defaults false (safer)."
  default     = false
}

variable "versioning" {
  type        = bool
  description = "Enable blob versioning (recommended for non-throwaway storage). Maps from the provider-neutral S3Spec.Versioning."
  default     = true
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
