variable "region" {
  type        = string
  description = "AWS region."
}

variable "name" {
  type        = string
  description = "Secret name (path-like; e.g. prod/api/jwt-key)."
}

variable "description" {
  type        = string
  description = "What this secret holds (for ops / audit)."
  default     = ""
}

variable "kms_key_arn" {
  type        = string
  description = "Customer-managed KMS key ARN. Empty = AWS-managed aws/secretsmanager (free)."
  default     = ""
}

variable "recovery_window_days" {
  type        = number
  description = "Soft-delete grace period (7-30). 0 = force-delete (use only in dev/lab)."
  default     = 7

  validation {
    condition     = var.recovery_window_days == 0 || (var.recovery_window_days >= 7 && var.recovery_window_days <= 30)
    error_message = "recovery_window_days must be 0 (force delete) or between 7 and 30."
  }
}

variable "rotation_lambda_arn" {
  type        = string
  description = "Lambda that rotates the secret. Empty = no automatic rotation."
  default     = ""
}

variable "rotation_days" {
  type        = number
  description = "Days between automatic rotations. Ignored when rotation_lambda_arn is empty."
  default     = 30
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto the secret."
  default     = {}
}
