variable "region" {
  type        = string
  description = "Home region for the trail/detectors."
}

variable "assume_role_arn" {
  type        = string
  description = "Role ARN in the member account to assume."
}

variable "name" {
  type        = string
  description = "Name prefix (e.g. opord-<csa_id>)."
}

variable "cloudtrail_retention_days" {
  type        = number
  description = "Days to retain CloudTrail logs in S3 before expiration."
  default     = 365
}

variable "enable_securityhub_cis" {
  type        = bool
  description = "Subscribe to the CIS AWS Foundations standard (in addition to AWS Foundational Best Practices)."
  default     = true
}

variable "tags" {
  type        = map(string)
  description = "Tags applied to all resources."
  default     = {}
}
