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

variable "template_vmid" {
  type        = number
  description = "VMID of the template to clone."
}

# --- Cluster sizing ---
variable "control_plane_count" {
  type        = number
  description = "Number of control-plane nodes (odd, >= 1)."
  default     = 3

  validation {
    condition     = var.control_plane_count >= 1 && var.control_plane_count % 2 == 1
    error_message = "control_plane_count must be an odd number >= 1."
  }
}

variable "worker_count" {
  type        = number
  description = "Number of worker nodes (>= 1)."
  default     = 3

  validation {
    condition     = var.worker_count >= 1
    error_message = "worker_count must be >= 1."
  }
}

variable "control_plane_specs" {
  type = object({
    cpu    = number
    memory = number # MB
    disk   = number # GB
  })
  default = {
    cpu    = 2
    memory = 4096
    disk   = 40
  }
}

variable "worker_specs" {
  type = object({
    cpu    = number
    memory = number # MB
    disk   = number # GB
  })
  default = {
    cpu    = 2
    memory = 4096
    disk   = 40
  }
}

# --- Naming ---
variable "cp_name_prefix" {
  type    = string
  default = "k8s-cp"
}

variable "worker_name_prefix" {
  type    = string
  default = "k8s-worker"
}

# --- Networking ---
variable "cp_ip_start" {
  type        = string
  description = "First control-plane IPv4 address."
}

variable "worker_ip_start" {
  type        = string
  description = "First worker IPv4 address."
}

variable "netmask_bits" {
  type    = number
  default = 24
}

variable "gateway" {
  type = string
}

variable "dns_servers" {
  type    = list(string)
  default = []
}

variable "dns_domain" {
  type    = string
  default = "cluster.local"
}

variable "control_plane_endpoint" {
  type        = string
  description = "Kubernetes API endpoint (VIP or first control plane)."
}

variable "control_plane_endpoint_port" {
  type    = number
  default = 6443
}

# --- SSH / cloud-init ---
variable "ssh_user" {
  type    = string
  default = "debian"
}

variable "ssh_public_key" {
  type    = string
  default = ""
}

variable "environment" {
  type    = string
  default = "dev"
}
