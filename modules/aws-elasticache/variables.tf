variable "region" {
  type        = string
  description = "AWS region."
}

variable "name" {
  type        = string
  description = "Replication group id (lower-case, 1-40 chars, used as cluster name)."

  validation {
    condition     = length(var.name) >= 1 && length(var.name) <= 40
    error_message = "name must be 1-40 chars."
  }
}

variable "engine_version" {
  type        = string
  description = "Redis engine version (e.g. 7.1, 7.0)."
  default     = "7.1"
}

variable "node_type" {
  type        = string
  description = "Cache node instance type. cache.t4g.micro is the cheapest for dev."
  default     = "cache.t4g.micro"
}

variable "num_cache_nodes" {
  type        = number
  description = "Number of nodes (1 = single node; >1 enables automatic failover)."
  default     = 1

  validation {
    condition     = var.num_cache_nodes >= 1 && var.num_cache_nodes <= 6
    error_message = "num_cache_nodes must be 1-6."
  }
}

variable "parameter_group_name" {
  type        = string
  description = "Cache parameter group. Empty = AWS default for the engine major."
  default     = ""
}

variable "subnet_ids" {
  type        = list(string)
  description = "Private subnet ids (at least 2 AZs for failover). Required."
}

variable "security_group_ids" {
  type        = list(string)
  description = "Existing security groups. Empty = module creates a VPC-CIDR-scoped one."
  default     = []
}

variable "at_rest_encryption" {
  type        = bool
  description = "Encrypt data on disk."
  default     = true
}

variable "in_transit_encryption" {
  type        = bool
  description = "TLS between clients and the cluster (needed for auth_token)."
  default     = true
}

variable "auth_token" {
  type        = string
  description = "Redis AUTH token (16-128 chars). Requires in_transit_encryption=true."
  default     = ""
  sensitive   = true
}

variable "tags" {
  type        = map(string)
  description = "Extra tags."
  default     = {}
}
