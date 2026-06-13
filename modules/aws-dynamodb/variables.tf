variable "region" {
  type        = string
  description = "AWS region, e.g. eu-central-1."
}

variable "name" {
  type        = string
  description = "DynamoDB table name."
}

variable "hash_key" {
  type        = string
  description = "Partition (hash) key attribute name."
}

variable "hash_key_type" {
  type        = string
  description = "Hash key attribute type: S (string), N (number), or B (binary)."
  default     = "S"
}

variable "range_key" {
  type        = string
  description = "Sort (range) key attribute name (empty = none)."
  default     = ""
}

variable "range_key_type" {
  type        = string
  description = "Range key attribute type: S, N, or B."
  default     = "S"
}

variable "billing_mode" {
  type        = string
  description = "PAY_PER_REQUEST (on-demand) or PROVISIONED."
  default     = "PAY_PER_REQUEST"
}

variable "read_capacity" {
  type        = number
  description = "Read capacity units (only for PROVISIONED)."
  default     = 0
}

variable "write_capacity" {
  type        = number
  description = "Write capacity units (only for PROVISIONED)."
  default     = 0
}

variable "environment" {
  type        = string
  description = "Environment label (dev, test, production)."
  default     = "dev"
}
