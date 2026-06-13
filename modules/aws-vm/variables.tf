# --- AWS placement ---
variable "region" {
  type        = string
  description = "AWS region, e.g. eu-central-1."
}

variable "subnet_id" {
  type        = string
  description = "Subnet to launch into (empty = account default subnet)."
  default     = ""
}

variable "security_group_ids" {
  type        = list(string)
  description = "Security groups to attach (empty = default)."
  default     = []
}

variable "key_name" {
  type        = string
  description = "EC2 key pair name for SSH (empty = none)."
  default     = ""
}

# --- Image / sizing ---
variable "ami" {
  type        = string
  description = "AMI ID to launch (e.g. ami-0abc...)."
}

variable "instance_type" {
  type        = string
  description = "EC2 instance type (e.g. t3.medium)."
  default     = "t3.medium"
}

variable "vm_count" {
  type        = number
  description = "Number of instances to create."
  default     = 1

  validation {
    condition     = var.vm_count >= 1
    error_message = "vm_count must be >= 1."
  }
}

variable "name_prefix" {
  type        = string
  description = "Instance Name-tag prefix; instances get -01, -02, ..."
  default     = "vm"
}

variable "root_volume_gb" {
  type        = number
  description = "Root EBS volume size in GB."
  default     = 40
}

variable "associate_public_ip" {
  type        = bool
  description = "Assign a public IP."
  default     = false
}

variable "environment" {
  type        = string
  description = "Environment label (dev, test, production)."
  default     = "dev"
}
