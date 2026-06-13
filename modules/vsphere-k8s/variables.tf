# --- vSphere connection ---
variable "vsphere_server" {
  type        = string
  description = "vCenter server FQDN or IP."
}

variable "vsphere_user" {
  type        = string
  description = "vCenter username."
}

variable "vsphere_password" {
  type        = string
  description = "vCenter password."
  sensitive   = true
}

variable "vsphere_allow_unverified_ssl" {
  type        = bool
  description = "Allow self-signed/unverified vCenter TLS certificates."
  default     = true
}

# --- vSphere placement ---
variable "vsphere_datacenter" {
  type        = string
  description = "Datacenter name."
}

variable "vsphere_cluster" {
  type        = string
  description = "Compute cluster name (its root resource pool is used)."
}

variable "vsphere_datastore" {
  type        = string
  description = "Datastore name for VM disks."
}

variable "vsphere_network" {
  type        = string
  description = "Port group / network name to attach VMs to."
}

variable "vsphere_folder_path" {
  type        = string
  description = "VM folder path relative to the datacenter (empty = datacenter root)."
  default     = ""
}

# --- Template ---
variable "template_name" {
  type        = string
  description = "Name of the golden VM template to clone (carries the OS, cloud-init, and the SSH key)."
}

variable "firmware" {
  type        = string
  description = "VM firmware; must match the template."
  default     = "efi"
}

# --- Sizing ---
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
  description = "Control-plane node hardware sizing."
  default = {
    cpu    = 4
    memory = 8192
    disk   = 50
  }
}

variable "worker_specs" {
  type = object({
    cpu    = number
    memory = number # MB
    disk   = number # GB
  })
  description = "Worker node hardware sizing."
  default = {
    cpu    = 4
    memory = 8192
    disk   = 50
  }
}

variable "worker_data_disks" {
  type        = list(number)
  description = "Additional data-disk sizes (GB) attached to each worker."
  default     = []
}

# --- Naming ---
variable "cluster_name" {
  type        = string
  description = "Short cluster identifier (used for tagging)."
  default     = "k8s"
}

variable "cp_name_prefix" {
  type        = string
  description = "Control-plane VM name prefix; nodes get -01, -02, ..."
  default     = "k8s-cp"
}

variable "worker_name_prefix" {
  type        = string
  description = "Worker VM name prefix; nodes get -01, -02, ..."
  default     = "k8s-worker"
}

# --- Networking ---
variable "cp_ip_start" {
  type        = string
  description = "First control-plane IPv4 address; subsequent nodes increment the last octet."
}

variable "worker_ip_start" {
  type        = string
  description = "First worker IPv4 address; subsequent nodes increment the last octet."
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
}

variable "dns_suffix" {
  type        = string
  description = "DNS search domain / node domain."
  default     = "cluster.local"
}

variable "control_plane_endpoint" {
  type        = string
  description = "Kubernetes API endpoint (VIP or first control plane)."
}

variable "control_plane_endpoint_port" {
  type        = number
  description = "Kubernetes API port."
  default     = 6443
}

# --- SSH (informational; the golden template carries the authorized key) ---
variable "ssh_user" {
  type        = string
  description = "SSH/login user baked into the template (used by Ansible)."
  default     = "debian"
}

variable "ssh_public_key" {
  type        = string
  description = "SSH public key (recorded for reference; injection is handled by the template/cloud-init)."
  default     = ""
}

variable "environment" {
  type        = string
  description = "Environment label (dev, test, production)."
  default     = "dev"
}
