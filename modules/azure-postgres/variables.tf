variable "location" {
  type        = string
  description = "Azure region."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix. Server name has tight rules (3-63 chars, lowercase + digits + hyphens)."
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}

variable "postgres_version" {
  type        = string
  description = "Postgres major version (Azure-supported: 11, 12, 13, 14, 15, 16)."
  default     = "16"
}

variable "sku_name" {
  type        = string
  description = "Flexible Server SKU. B_Standard_B1ms = cheapest burstable (~$13/mo). For prod use GP_Standard_D2s_v3+."
  default     = "B_Standard_B1ms"
}

variable "storage_mb" {
  type        = number
  description = "Storage in MB (32768 = 32 GB minimum)."
  default     = 32768
}

variable "admin_username" {
  type        = string
  description = "Postgres admin username."
  default     = "opordadmin"
}

variable "database_name" {
  type        = string
  description = "Optional initial database to create. Empty = none (only the postgres default)."
  default     = ""
}

variable "allow_public_access" {
  type        = bool
  description = "Allow connections from anywhere (firewall rule 0.0.0.0-255.255.255.255). DEV ONLY."
  default     = false
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
