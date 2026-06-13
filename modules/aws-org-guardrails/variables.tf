variable "region" {
  type        = string
  description = "AWS region for the provider (Organizations is global; region is still required)."
  default     = "us-east-1"
}

variable "target_ids" {
  type        = list(string)
  description = "OU or root IDs to attach the SCP + tag policy to (e.g. the OU holding provisioned member accounts)."

  validation {
    condition     = length(var.target_ids) > 0
    error_message = "Provide at least one target OU/root ID to attach guardrails to."
  }
}

variable "name_prefix" {
  type        = string
  description = "Prefix for policy names."
  default     = "opord-"
}

variable "enable_region_lock" {
  type        = bool
  description = "Deny actions outside allowed_regions (global services are always exempt)."
  default     = false
}

variable "allowed_regions" {
  type        = list(string)
  description = "Regions members may operate in when enable_region_lock = true."
  default     = ["eu-central-1", "us-east-1"]
}

variable "enable_tag_policy" {
  type        = bool
  description = "Attach a tag policy enforcing the required org tags."
  default     = true
}

variable "required_tag_keys" {
  type        = list(string)
  description = "Tag keys the tag policy enforces on taggable resources."
  default     = ["csa_id", "owner", "environment"]
}
