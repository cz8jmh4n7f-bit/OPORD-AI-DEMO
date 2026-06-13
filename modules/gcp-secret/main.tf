variable "name" {
  type        = string
  description = "Secret id (the container name)."
}

variable "labels" {
  type    = map(string)
  default = {}
}

# Secret Manager secret CONTAINER only. The plaintext value is added out-of-band
# (console / gcloud / Vault-sync), so OPORD never holds it.
resource "google_secret_manager_secret" "this" {
  secret_id = var.name
  labels    = var.labels

  replication {
    auto {}
  }
}

output "secret_id" {
  value       = google_secret_manager_secret.this.secret_id
  description = "The short secret id."
}

output "secret_arn" {
  value       = google_secret_manager_secret.this.id
  description = "GCP has no ARN; the full resource id (projects/.../secrets/...) is the stable identifier."
}

output "name" {
  value = google_secret_manager_secret.this.name
}

output "uri" {
  value       = google_secret_manager_secret.this.id
  description = "Full resource id, usable with gcloud secrets versions add."
}
