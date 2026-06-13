variable "name" {
  description = "The domain name for the hosted zone (e.g. example.com)."
  type        = string
}

variable "private" {
  description = "Create a private (VPC-associated) hosted zone instead of a public one."
  type        = bool
  default     = false
}

variable "vpc_id" {
  description = "VPC to associate with a private hosted zone. Required when private = true."
  type        = string
  default     = ""
}

variable "records" {
  description = "Optional DNS records to create in the zone."
  type = list(object({
    name  = string
    type  = string
    value = string
    alias = bool
    ttl   = number
  }))
  default = []
}

variable "region" {
  description = "AWS region. Accepted for consistency with other OPORD modules; the provider region is set by the ambient AWS_REGION."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags to apply to the hosted zone."
  type        = map(string)
  default     = {}
}
