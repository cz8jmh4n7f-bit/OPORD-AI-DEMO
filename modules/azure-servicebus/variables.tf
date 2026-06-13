variable "location" {
  type        = string
  description = "Azure region."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix; namespace name has tight rules (6-50 chars, alphanumeric + hyphens, globally unique)."
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}

variable "sku" {
  type        = string
  description = "Namespace SKU: Basic / Standard / Premium. Standard = queues+topics, Premium = VNet."
  default     = "Standard"
  validation {
    condition     = contains(["Basic", "Standard", "Premium"], var.sku)
    error_message = "sku must be Basic, Standard, or Premium."
  }
}

variable "queue_names" {
  type        = list(string)
  description = "Queue names to create. Empty = namespace only."
  default     = []
}

variable "max_size_megabytes" {
  type        = number
  description = "Max queue size (1024 - 81920 for Standard; up to 81920 for Premium)."
  default     = 1024
}

variable "lock_duration" {
  type        = string
  description = "Message lock duration (ISO-8601, e.g. PT30S = 30 seconds)."
  default     = "PT30S"
}

variable "enable_dead_lettering" {
  type        = bool
  description = "Enable dead-letter on message expiration."
  default     = true
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
