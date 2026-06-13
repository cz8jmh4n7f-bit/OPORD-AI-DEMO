variable "name" {
  type = string
}

variable "region" {
  type    = string
  default = "europe-west1"
}

variable "memory_size_gb" {
  type    = number
  default = 1
}

variable "redis_version" {
  type    = string
  default = "REDIS_7_0"
}

variable "replicated" {
  type        = bool
  default     = false
  description = "STANDARD_HA (replicated, multi-AZ) when true, else BASIC (single node)."
}

variable "transit_encryption" {
  type    = bool
  default = true
}

variable "auth_enabled" {
  type    = bool
  default = true
}

variable "labels" {
  type    = map(string)
  default = {}
}

variable "authorized_network" {
  type        = string
  default     = ""
  description = "VPC self-link/name Memorystore attaches to. Empty = auto-detect the factory VPC (opord-*-vpc) when the project has one (deploy-into a governed project whose 'default' network was removed), else 'default'."
}

resource "google_redis_instance" "this" {
  name                    = var.name
  tier                    = var.replicated ? "STANDARD_HA" : "BASIC"
  memory_size_gb          = var.memory_size_gb
  region                  = var.region
  redis_version           = var.redis_version
  auth_enabled            = var.auth_enabled
  transit_encryption_mode = var.transit_encryption ? "SERVER_AUTHENTICATION" : "DISABLED"
  authorized_network      = var.authorized_network != "" ? var.authorized_network : null
  labels                  = var.labels
}

output "primary_endpoint" {
  value = google_redis_instance.this.host
}

output "reader_endpoint" {
  value       = google_redis_instance.this.read_endpoint
  description = "Read replica endpoint (STANDARD_HA only; empty for BASIC)."
}

output "port" {
  value = google_redis_instance.this.port
}

output "id" {
  value = google_redis_instance.this.id
}
