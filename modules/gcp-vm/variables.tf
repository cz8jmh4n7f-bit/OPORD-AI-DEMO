variable "region" {
  type    = string
  default = "europe-west1"
}

variable "zone" {
  type    = string
  default = "europe-west1-b"
}

variable "name_prefix" {
  type        = string
  description = "Name base for the VPC, subnet, firewall and instances."
}

variable "environment" {
  type    = string
  default = "dev"
}

variable "vm_count" {
  type    = number
  default = 1
}

variable "machine_type" {
  type    = string
  default = "e2-micro"
}

variable "image" {
  type        = string
  description = "Boot image (family shorthand, e.g. ubuntu-os-cloud/ubuntu-2204-lts)."
  default     = "ubuntu-os-cloud/ubuntu-2204-lts"
}

variable "ssh_user" {
  type    = string
  default = "opord"
}

variable "ssh_public_key" {
  type        = string
  description = "SSH public key added to instance metadata. Empty = no SSH key (use OS Login / IAP)."
  default     = ""
}

variable "disk_gb" {
  type    = number
  default = 10
}

variable "public_ip" {
  type        = bool
  description = "Attach an ephemeral external IP."
  default     = false
}

variable "allow_ssh_from" {
  type        = list(string)
  description = "Source CIDRs allowed to reach TCP 22."
  default     = ["0.0.0.0/0"]
}

variable "labels" {
  type    = map(string)
  default = {}
}
