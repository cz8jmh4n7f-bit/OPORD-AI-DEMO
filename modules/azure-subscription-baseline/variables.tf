variable "subscription_id" {
  type        = string
  description = "Target subscription GUID. L1 output."
}

variable "location" {
  type        = string
  description = "Default Azure region for the base resource groups."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "OPORD naming prefix (e.g. opord)."
  default     = "opord"
}

variable "csa_id" {
  type        = string
  description = "Customer/project identifier."
}

variable "csa_cloud_name" {
  type        = string
  description = "Environment classifier (prod, stage, dev)."
  default     = "dev"
}

variable "resource_providers" {
  type        = list(string)
  description = <<-EOT
    Resource providers to explicitly register on the subscription. Default
    is empty: Azure auto-registers RPs on first resource create, and the
    azurerm v4 azurerm_resource_provider_registration validator rejects
    some legitimate names (e.g. Microsoft.Insights). Opt in by supplying
    a list if you need explicit pre-registration (compliance/audit).
  EOT
  default = []
}

variable "defender_plans_standard" {
  type        = list(string)
  description = <<-EOT
    Defender for Cloud plans to enable at Standard (paid) tier. Empty list to 
    Free tier only (CSPM-only, ~$0/mo idle). Common values once you opt in:
    VirtualMachines / SqlServers / KeyVaults / StorageAccounts / Containers.
  EOT
  default     = []
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
