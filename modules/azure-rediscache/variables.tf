variable "location" {
  type    = string
  default = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Cache name base; the Redis cache (a global DNS name) is derived from it."
}

variable "environment" {
  type    = string
  default = "dev"
}

variable "sku_name" {
  type        = string
  description = "Basic (single node, no SLA), Standard (replicated), or Premium."
  default     = "Basic"
}

variable "family" {
  type        = string
  description = "C for Basic/Standard, P for Premium."
  default     = "C"
}

variable "capacity" {
  type        = number
  description = "Size: 0-6 for C (0 = 250MB), 1-5 for P."
  default     = 0
}

variable "redis_version" {
  type        = string
  description = "Redis major version (e.g. 6)."
  default     = "6"
}

variable "tags" {
  type    = map(string)
  default = {}
}
