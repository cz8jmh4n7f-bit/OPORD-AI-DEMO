variable "domain" {
  description = "Primary domain name for the certificate (CN)."
  type        = string
}

variable "subject_alternative_names" {
  description = "Additional domain names (SANs) covered by the certificate."
  type        = list(string)
  default     = []
}

variable "validation_zone_id" {
  description = "Route53 hosted-zone id. When set, the module creates the DNS validation records and waits for the certificate to validate; when empty, the cert is only requested (stays PENDING_VALIDATION until the domain owner validates it)."
  type        = string
  default     = ""
}

variable "region" {
  description = "AWS region. Empty means use the ambient AWS_REGION OPORD injects."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Extra tags applied to the certificate."
  type        = map(string)
  default     = {}
}
