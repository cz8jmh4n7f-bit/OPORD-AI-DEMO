# --- Proxmox connection ---
variable "proxmox_endpoint" {
  type        = string
  description = "Proxmox API endpoint, e.g. https://pve.example.com:8006/"
}

variable "proxmox_username" {
  type        = string
  description = "Proxmox username, e.g. root@pam"
}

variable "proxmox_password" {
  type        = string
  description = "Proxmox password."
  sensitive   = true
}

variable "proxmox_insecure" {
  type        = bool
  description = "Allow self-signed/unverified Proxmox TLS certificates."
  default     = true
}

# --- Placement ---
variable "node_name" {
  type        = string
  description = "Proxmox node to create the VMs on."
}

variable "datastore_id" {
  type        = string
  description = "Storage for VM disks."
  default     = "local-lvm"
}

variable "network_bridge" {
  type        = string
  description = "Network bridge to attach VMs to."
  default     = "vmbr0"
}

# --- Template ---
variable "template_vmid" {
  type        = number
  description = "VMID of the template to clone."
}

# --- Sizing ---
variable "vm_count" {
  type        = number
  description = "Number of VMs to create."
  default     = 1

  validation {
    condition     = var.vm_count >= 1
    error_message = "vm_count must be >= 1."
  }
}

variable "name_prefix" {
  type        = string
  description = "VM name prefix; instances get -01, -02, ..."
  default     = "vm"
}

variable "cores" {
  type        = number
  description = "vCPU cores per VM."
  default     = 2
}

variable "memory_mb" {
  type        = number
  description = "Memory per VM in MB."
  default     = 4096
}

variable "disk_gb" {
  type        = number
  description = "Primary disk size per VM in GB."
  default     = 40
}

# --- Networking ---
variable "ip_start" {
  type        = string
  description = "First IPv4 address; subsequent VMs increment the last octet."
}

variable "netmask_bits" {
  type        = number
  description = "IPv4 prefix length (e.g. 24)."
  default     = 24
}

variable "gateway" {
  type        = string
  description = "Default gateway IPv4 address."
}

variable "dns_servers" {
  type        = list(string)
  description = "DNS server IPs."
  default     = []
}

variable "dns_domain" {
  type        = string
  description = "DNS search domain."
  default     = "local"
}

# --- SSH / cloud-init ---
variable "ssh_user" {
  type        = string
  description = "cloud-init user to create."
  default     = "debian"
}

variable "ssh_public_key" {
  type        = string
  description = "SSH public key injected via cloud-init."
  default     = ""
}

variable "environment" {
  type        = string
  description = "Environment label (dev, test, production)."
  default     = "dev"
}
