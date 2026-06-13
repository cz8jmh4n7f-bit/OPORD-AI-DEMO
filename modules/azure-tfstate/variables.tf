variable "location" {
  type        = string
  description = "Azure region for the Storage Account hosting tfstate."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix used for the RG and storage account. The SA name is derived as <name_prefix>tfstate (lowercase, alphanumeric only, capped to 24 chars including the 'tfstate' suffix)."
}

variable "container_name" {
  type        = string
  description = "Blob container name where state files live."
  default     = "tfstate"
}

variable "soft_delete_retention_days" {
  type        = number
  description = "Days to retain soft-deleted blobs (1-365). Versioning + soft delete are the operator's safety net against accidental state loss."
  default     = 30
  validation {
    condition     = var.soft_delete_retention_days >= 1 && var.soft_delete_retention_days <= 365
    error_message = "soft_delete_retention_days must be between 1 and 365."
  }
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
