variable "location" {
  type        = string
  description = "Azure region."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix. Cluster name: 1-63 chars, lowercase + digits + hyphens."
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}

variable "kubernetes_version" {
  type        = string
  description = "AKS Kubernetes version (e.g. 1.30, 1.29). Empty = AKS default."
  default     = ""
}

variable "node_count" {
  type        = number
  description = "Number of nodes in the system pool."
  default     = 1
  validation {
    condition     = var.node_count >= 1 && var.node_count <= 50
    error_message = "node_count must be between 1 and 50."
  }
}

variable "node_vm_size" {
  type        = string
  description = "VM size for nodes. Standard_B2s = cheapest 2 vCPU/4 GB burstable."
  default     = "Standard_B2s"
}

variable "node_disk_gb" {
  type        = number
  description = "OS disk size per node in GB (30-2048)."
  default     = 30
}

variable "dns_prefix" {
  type        = string
  description = "DNS prefix for the API server (1-54 chars, must start with letter; module derives one from name_prefix when empty)."
  default     = ""
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
