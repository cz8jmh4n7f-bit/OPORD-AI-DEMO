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
  description = "Name of the golden VM template to clone."
}

variable "firmware" {
  type        = string
  description = "VM firmware; must match the template."
  default     = "efi"
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

variable "specs" {
  type = object({
    cpu    = number
    memory = number # MB
    disk   = number # GB (primary disk)
  })
  description = "Per-VM hardware sizing."
  default = {
    cpu    = 2
    memory = 4096
    disk   = 40
  }
}

variable "data_disks" {
  type        = list(number)
  description = "Additional data-disk sizes (GB) attached to each VM."
  default     = []
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
}

variable "dns_suffix" {
  type        = string
  description = "DNS search domain / node domain."
  default     = "local"
}

# --- SSH (informational; the golden template carries the authorized key) ---
variable "ssh_user" {
  type        = string
  description = "SSH/login user baked into the template."
  default     = "debian"
}

variable "ssh_public_key" {
  type        = string
  description = "SSH public key (recorded for reference; injection handled by the template)."
  default     = ""
}

variable "environment" {
  type        = string
  description = "Environment label (dev, test, production)."
  default     = "dev"
}
