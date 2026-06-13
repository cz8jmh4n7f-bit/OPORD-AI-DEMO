variable "name" {
  type = string
}

variable "region" {
  type    = string
  default = "europe-west1"
}

variable "database_version" {
  type    = string
  default = "POSTGRES_16"
}

variable "tier" {
  type        = string
  default     = "db-f1-micro"
  description = "Cloud SQL machine type (db-f1-micro / db-g1-small / db-custom-N-M...)."
}

variable "disk_gb" {
  type    = number
  default = 10
}

variable "db_name" {
  type    = string
  default = "opord"
}

variable "username" {
  type    = string
  default = "opord"
}

variable "iam_auth" {
  type        = bool
  default     = false
  description = "Passwordless IAM database authentication (no static password)."
}

variable "iam_principal" {
  type        = string
  default     = ""
  description = "IAM user email or service-account email granted DB access when iam_auth=true."
}

variable "public_access" {
  type        = bool
  default     = false
  description = "When true, opens an authorized network of 0.0.0.0/0 (dev only)."
}

variable "labels" {
  type    = map(string)
  default = {}
}

resource "random_password" "pw" {
  count   = var.iam_auth ? 0 : 1
  length  = 20
  special = false
}

resource "google_sql_database_instance" "this" {
  name                = var.name
  region              = var.region
  database_version    = var.database_version
  deletion_protection = false

  settings {
    # ENTERPRISE supports the shared-core tiers OPORD defaults to (db-f1-micro /
    # db-g1-small). ENTERPRISE_PLUS only allows db-perf-optimized-* tiers, so the
    # default instance would reject db-f1-micro.
    edition           = "ENTERPRISE"
    tier              = var.tier
    disk_size         = var.disk_gb
    disk_type         = "PD_SSD"
    availability_type = "ZONAL"
    user_labels       = var.labels

    dynamic "database_flags" {
      for_each = var.iam_auth ? [1] : []
      content {
        name  = "cloudsql.iam_authentication"
        value = "on"
      }
    }

    ip_configuration {
      # A public IP is always allocated so there is a reachable endpoint; access
      # is firewalled unless public_access opens it (a private VPC peering is a
      # later enhancement).
      ipv4_enabled = true

      dynamic "authorized_networks" {
        for_each = var.public_access ? [1] : []
        content {
          name  = "all"
          value = "0.0.0.0/0"
        }
      }
    }
  }
}

resource "google_sql_database" "this" {
  name     = var.db_name
  instance = google_sql_database_instance.this.name
}

resource "google_sql_user" "this" {
  count    = var.iam_auth ? 0 : 1
  name     = var.username
  instance = google_sql_database_instance.this.name
  password = random_password.pw[0].result
}

# Passwordless: an IAM-authenticated user - CLOUD_IAM_USER for a person, or
# CLOUD_IAM_SERVICE_ACCOUNT for a SA (Cloud SQL truncates the .gserviceaccount.com
# suffix for SAs). The principal connects with a short-lived IAM token, no password.
resource "google_sql_user" "iam" {
  count    = var.iam_auth ? 1 : 0
  name     = endswith(var.iam_principal, "gserviceaccount.com") ? trimsuffix(var.iam_principal, ".gserviceaccount.com") : var.iam_principal
  instance = google_sql_database_instance.this.name
  type     = endswith(var.iam_principal, "gserviceaccount.com") ? "CLOUD_IAM_SERVICE_ACCOUNT" : "CLOUD_IAM_USER"
}

output "endpoint" {
  value = google_sql_database_instance.this.public_ip_address
}

output "port" {
  value = startswith(var.database_version, "MYSQL") ? 3306 : 5432
}

output "instance" {
  value = google_sql_database_instance.this.name
}

output "connection_name" {
  value = google_sql_database_instance.this.connection_name
}

output "password" {
  value       = var.iam_auth ? "" : random_password.pw[0].result
  description = "Generated DB user password (empty when iam_auth). Also in tofu state; OPORD stores it in the secrets store (OpenBao)."
  sensitive   = true
}

output "auth_mode" {
  value = var.iam_auth ? "iam" : "password"
}

output "iam_principal" {
  value = var.iam_auth ? var.iam_principal : ""
}
