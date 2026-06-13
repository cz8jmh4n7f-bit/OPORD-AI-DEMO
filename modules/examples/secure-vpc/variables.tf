variable "region" {
  type        = string
  description = "AWS region for the VPC."
}

variable "name" {
  type        = string
  description = "Name prefix for VPC resources (e.g. opord-vpctest)."
}

variable "vpc_cidr" {
  type        = string
  description = "VPC CIDR - a /22 (in the account factory this comes from the Vault CIDR pool)."

  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0)) && tonumber(split("/", var.vpc_cidr)[1]) == 22
    error_message = "vpc_cidr must be a valid /22 block."
  }
}

variable "az_count" {
  type        = number
  description = "Number of AZs / subnets to spread across."
  default     = 3

  validation {
    condition     = var.az_count >= 2 && var.az_count <= 4
    error_message = "az_count must be between 2 and 4 (a /22 splits into 4 /24s)."
  }
}

variable "flow_log_retention_days" {
  type        = number
  description = "CloudWatch retention for VPC Flow Logs."
  default     = 90
}

variable "tags" {
  type        = map(string)
  description = "Tags applied to all resources."
  default     = {}
}
