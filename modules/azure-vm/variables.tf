variable "location" {
  type        = string
  description = "Azure region (e.g. westeurope, northeurope, eastus)."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix for all resources (RG, NIC, VM, ...). Must be valid Azure naming."
}

variable "environment" {
  type        = string
  description = "Environment tag (dev/staging/prod)."
  default     = "dev"
}

variable "vm_count" {
  type        = number
  description = "Number of VMs to create."
  default     = 1

  validation {
    condition     = var.vm_count >= 1 && var.vm_count <= 20
    error_message = "vm_count must be between 1 and 20."
  }
}

variable "vm_size" {
  type        = string
  description = "Azure VM SKU. Standard_B1s is the cheapest dev option."
  default     = "Standard_B1s"
}

variable "admin_username" {
  type        = string
  description = "Linux admin username."
  default     = "azureuser"
}

variable "ssh_public_key" {
  type        = string
  description = "SSH public key (OpenSSH format) for the admin user. REQUIRED - no password auth."

  validation {
    condition     = length(var.ssh_public_key) > 0
    error_message = "ssh_public_key is required (Linux SSH-only access)."
  }
}

variable "image" {
  type = object({
    publisher = string
    offer     = string
    sku       = string
    version   = string
  })
  description = "Marketplace image. Default: Ubuntu 22.04 LTS."
  default = {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts-gen2"
    version   = "latest"
  }
}

variable "os_disk_gb" {
  type        = number
  description = "OS disk size in GB."
  default     = 30
}

variable "subnet_id" {
  type        = string
  description = "Existing subnet ID. Empty = module creates a /24 VNet + subnet (recommended for isolated test VMs)."
  default     = ""
}

variable "vnet_cidr" {
  type        = string
  description = "CIDR for the auto-created VNet (ignored when subnet_id is set)."
  default     = "10.30.0.0/24"
}

variable "associate_public_ip" {
  type        = bool
  description = "Allocate a Public IP and attach to the NIC. Set false for private-only VMs."
  default     = true
}

variable "allow_ssh_from" {
  type        = list(string)
  description = "CIDRs allowed to SSH on port 22. Default: anywhere (dev). Tighten for prod."
  default     = ["0.0.0.0/0"]
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
