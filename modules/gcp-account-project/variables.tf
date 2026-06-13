variable "csa_id" {
  type        = string
  description = "Unique CSA id; drives the project id + resource naming."
  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,20}$", var.csa_id))
    error_message = "csa_id must be lowercase letters/numbers/dashes, start with a letter, 2-21 chars."
  }
}

variable "cloud_name" {
  type        = string
  description = "Environment: prod / stage / dev. Part of the project id."
  validation {
    condition     = can(regex("^[a-z0-9]{1,6}$", var.cloud_name))
    error_message = "cloud_name must be lowercase letters/numbers, <=6 chars."
  }
}

variable "project_id_suffix" {
  type        = string
  default     = ""
  description = "Optional suffix appended to the project id for global-uniqueness collision avoidance (e.g. -a1b2). Project ids are globally unique + reserved 30 days after delete."
  validation {
    condition     = var.project_id_suffix == "" || can(regex("^[a-z0-9-]{1,8}$", var.project_id_suffix))
    error_message = "project_id_suffix must be lowercase letters/numbers/dashes, <=8 chars."
  }
}

variable "folder_parent" {
  type        = string
  description = "Parent for the per-CSA folder: 'organizations/NNN' or 'folders/NNN' (the project-factory parent)."
}

variable "billing_account" {
  type        = string
  description = "Billing account id (XXXXXX-XXXXXX-XXXXXX) to link to the project."
}

variable "owner" {
  type        = string
  default     = ""
  description = "Primary owner (used for the owner label; sanitized to label-safe form)."
}

variable "managed_by" {
  type    = string
  default = "opord"
}

variable "cost_center" {
  type        = string
  default     = ""
  description = "Cost-allocation center; defaults to csa_id."
}

variable "extra_labels" {
  type    = map(string)
  default = {}
}

variable "deletion_policy" {
  type        = string
  default     = "DELETE"
  description = "DELETE (dev; tofu destroy removes the project, 30-day reservation) or PREVENT (prod; destroy is blocked)."
  validation {
    condition     = contains(["DELETE", "PREVENT", "ABANDON"], var.deletion_policy)
    error_message = "deletion_policy must be DELETE, PREVENT, or ABANDON."
  }
}
