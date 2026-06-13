variable "subscription_id" {
  type        = string
  description = "Target subscription GUID (L1 output)."
}

variable "subscription_resource_id" {
  type        = string
  description = "Subscription ARM resource ID (L1 output). Scope for policy assignments."
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

variable "allowed_locations" {
  type        = list(string)
  description = "Azure regions allowed under the Allowed Locations policy. Resources outside this list are denied at apply time."
  default     = ["westeurope", "northeurope"]
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every assignment."
  default     = {}
}
