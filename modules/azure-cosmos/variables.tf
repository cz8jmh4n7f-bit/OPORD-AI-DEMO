variable "location" {
  type        = string
  description = "Azure region."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix; Cosmos account name has tight rules (3-44 chars, lowercase + digits + hyphens)."
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}

variable "consistency_level" {
  type        = string
  description = "Cosmos consistency: BoundedStaleness / Eventual / Session / Strong / ConsistentPrefix."
  default     = "Session"
}

variable "offer_type" {
  type        = string
  description = "Pricing tier. Only Standard is supported by Cosmos for SQL API."
  default     = "Standard"
}

variable "table_name" {
  type        = string
  description = "Cosmos container name (analogous to DynamoDB table). Required."
}

variable "partition_key" {
  type        = string
  description = "Container partition key path (e.g. /id or /tenantId). Required."
}

variable "billing_mode" {
  type        = string
  description = "Throughput model: SERVERLESS/PAY_PER_REQUEST, AUTOSCALE, or PROVISIONED fixed RU/s."
  default     = "SERVERLESS"
  validation {
    condition     = contains(["PAY_PER_REQUEST", "SERVERLESS", "AUTOSCALE", "PROVISIONED"], var.billing_mode)
    error_message = "billing_mode must be PAY_PER_REQUEST, SERVERLESS, AUTOSCALE, or PROVISIONED."
  }
}

variable "throughput" {
  type        = number
  description = "Manual RU/s when billing_mode=PROVISIONED. Ignored otherwise. Min 400, max 1M."
  default     = 400
}

variable "max_throughput" {
  type        = number
  description = "Autoscale max RU/s when billing_mode=AUTOSCALE. Ignored otherwise."
  default     = 4000
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
