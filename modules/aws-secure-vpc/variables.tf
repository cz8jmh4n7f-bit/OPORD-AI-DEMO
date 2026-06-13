variable "region" {
  type        = string
  description = "AWS region for the VPC."
}

variable "assume_role_arn" {
  type        = string
  description = "Role ARN in the member account to assume (e.g. OrganizationAccountAccessRole)."
}

variable "name" {
  type        = string
  description = "Name prefix for VPC resources (e.g. opord-<csa_id>)."
}

variable "vpc_cidr" {
  type        = string
  description = "VPC CIDR - a /22 allocated from the Vault CIDR pool."

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

variable "enable_nat" {
  type        = bool
  default     = false
  description = <<-EOT
    Add a NAT gateway so the private workload subnets get internet egress. Required
    for EKS managed node groups, which need to reach the EKS API / ECR / STS but get
    no public IP in a secure subnet (Ec2SubnetInvalidConfiguration otherwise). Off by
    default - the secure VPC is egress-free by design ($0, ZTNA). When true, the last
    /24 of the /22 becomes a public subnet for the NAT, so it needs a spare /24
    (az_count <= 3; the default 3 leaves the 4th /24 free). ~$32/mo per NAT.
  EOT
}

variable "tags" {
  type        = map(string)
  description = "Tags applied to all resources."
  default     = {}
}
