variable "name" {
  description = "Name of the load balancer and derived resources."
  type        = string
}

variable "internal" {
  description = "If true the ALB is internal (VPC-only); otherwise internet-facing."
  type        = bool
  default     = false
}

variable "subnet_ids" {
  description = "Subnets to place the ALB in (at least two, in different AZs)."
  type        = list(string)

  validation {
    condition     = length(var.subnet_ids) >= 2
    error_message = "An ALB requires at least two subnets in different availability zones."
  }
}

variable "security_group_ids" {
  description = "Security groups for the ALB. When empty, a VPC-CIDR-scoped SG opening the listener ports is auto-created."
  type        = list(string)
  default     = []
}

variable "listeners" {
  description = "Listeners to create. certificate_arn is required for HTTPS."
  type = list(object({
    port            = number
    protocol        = string
    certificate_arn = string
  }))
  default = [{
    port            = 80
    protocol        = "HTTP"
    certificate_arn = ""
  }]
}

variable "target_type" {
  description = "Target group target type: instance, ip, or lambda."
  type        = string
  default     = "instance"

  validation {
    condition     = contains(["instance", "ip", "lambda"], var.target_type)
    error_message = "target_type must be one of: instance, ip, lambda."
  }
}

variable "targets" {
  description = "Targets to register. instance/ip ids for those types; a single Lambda function ARN for lambda."
  type        = list(string)
  default     = []
}

variable "health_check_path" {
  description = "HTTP path the target group health check requests."
  type        = string
  default     = "/"
}

variable "region" {
  description = "AWS region. Empty falls back to the provider/ambient region."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Extra tags applied to all resources."
  type        = map(string)
  default     = {}
}
