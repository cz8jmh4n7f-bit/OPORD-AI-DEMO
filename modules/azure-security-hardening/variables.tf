variable "subscription_id" {
  type        = string
  description = "Target subscription GUID (L1 output)."
}

variable "subscription_resource_id" {
  type        = string
  description = "Subscription ARM resource ID (L1 output). Scope for the Activity Log Diagnostic Setting."
}

variable "logs_rg_name" {
  type        = string
  description = "Logs resource group name (L2 output)."
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

variable "law_retention_days" {
  type        = number
  description = "Log Analytics Workspace retention (30-730)."
  default     = 30
  validation {
    condition     = var.law_retention_days >= 30 && var.law_retention_days <= 730
    error_message = "law_retention_days must be between 30 and 730."
  }
}

variable "archive_storage_retention_days" {
  type        = number
  description = "Archive Storage blob soft-delete retention (1-365)."
  default     = 90
}

variable "cmk_versionless_id" {
  type        = string
  description = "Customer-Managed Key versionless ID from azure-keyvault-baseline. Empty to Microsoft-managed encryption (skip CMK wire-up)."
  default     = ""
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
