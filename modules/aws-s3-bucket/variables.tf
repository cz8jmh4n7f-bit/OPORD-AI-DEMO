variable "region" {
  type        = string
  description = "AWS region."
}

variable "name" {
  type        = string
  description = "Globally-unique bucket name (must follow S3 naming rules)."

  validation {
    condition     = length(var.name) >= 3 && length(var.name) <= 63
    error_message = "Bucket name must be 3-63 characters."
  }
}

variable "versioning" {
  type        = bool
  description = "Enable object versioning."
  default     = true
}

variable "block_public_access" {
  type        = bool
  description = "Block all forms of public access (4 BPA settings on)."
  default     = true
}

variable "kms_key_arn" {
  type        = string
  description = "KMS key ARN for SSE-KMS. Empty = SSE-S3 (AES256), free."
  default     = ""
}

variable "lifecycle_glacier_days" {
  type        = number
  description = "Move objects to Glacier Deep Archive after N days. 0 = no lifecycle rule."
  default     = 0
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto the bucket."
  default     = {}
}
