variable "subscription_id" {
  type        = string
  description = "Target subscription GUID (L1 output)."
}

variable "network_rg_name" {
  type        = string
  description = "Network resource group name (L2 output)."
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

variable "vnet_cidr" {
  type        = string
  description = "/22 block allocated by OPORD IPAM (Vault CAS pool). The module derives three /24 subnets inside it via cidrsubnet()."
  validation {
    condition     = can(cidrsubnet(var.vnet_cidr, 2, 0))
    error_message = "vnet_cidr must be a valid CIDR with enough room for three /24 subnets (e.g. 10.20.0.0/22)."
  }
}

variable "allow_inbound_cidrs" {
  type        = list(string)
  description = "Trusted CIDRs allowed to reach SSH/RDP/HTTPS/ICMP. Default 0.0.0.0/0 = dev. Tighten for prod."
  default     = ["0.0.0.0/0"]
}

variable "create_flow_logs" {
  type        = bool
  description = <<-EOT
    Create VNet Flow Logs (via the regional Network Watcher) writing to the
    archive Storage Account. Default true. Set false to skip Flow Logs - useful
    when the Network Watcher isn't available in the region, or to sidestep the
    teardown ordering where an active Flow Log blocks the VNet (and thus the
    network RG) from deleting. When false, flow_logs_storage_account_id is
    ignored.
  EOT
  default     = true
}

variable "flow_logs_storage_account_id" {
  type        = string
  description = "Storage Account ID where VNet Flow Logs are written. L5 output (security-hardening builds it with the CMK from KV baseline). Ignored when create_flow_logs=false."
  default     = ""
}

variable "flow_logs_retention_days" {
  type        = number
  description = "Days to retain Flow Logs (1-365)."
  default     = 30
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
