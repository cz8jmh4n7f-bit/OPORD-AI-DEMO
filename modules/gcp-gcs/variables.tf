variable "name" {
  type        = string
  description = "Base bucket name. A short random suffix is appended for global uniqueness."
}

variable "location" {
  type        = string
  default     = "EU"
  description = "Bucket location: a region (europe-west1) or a multi-region (EU/US/ASIA)."
}

variable "storage_class" {
  type        = string
  default     = "STANDARD"
  description = "Default storage class (STANDARD/NEARLINE/COLDLINE/ARCHIVE)."
}

variable "versioning" {
  type        = bool
  default     = true
  description = "Keep noncurrent object versions."
}

variable "block_public_access" {
  type        = bool
  default     = true
  description = "Enforce public access prevention (no public objects)."
}

variable "force_destroy" {
  type        = bool
  default     = true
  description = "Allow tofu destroy to delete a non-empty bucket (dev convenience)."
}

variable "archive_after_days" {
  type        = number
  default     = 0
  description = "If > 0, objects transition to the ARCHIVE class after N days."
}

variable "labels" {
  type        = map(string)
  default     = {}
  description = "Resource labels (keys/values lowercase; no colons)."
}
