variable "name" {
  type        = string
  description = "Firestore database id (named database)."
}

variable "location" {
  type        = string
  default     = "eur3"
  description = "Firestore location: a region (europe-west1) or multi-region (eur3/nam5)."
}

variable "mode" {
  type        = string
  default     = "FIRESTORE_NATIVE"
  description = "FIRESTORE_NATIVE or DATASTORE_MODE."
}

resource "google_firestore_database" "this" {
  name            = var.name
  location_id     = var.location
  type            = var.mode
  deletion_policy = "DELETE"
}

output "arn" {
  value       = google_firestore_database.this.id
  description = "The database resource id (projects/.../databases/...)."
}

output "name" {
  value = google_firestore_database.this.name
}

output "uid" {
  value = google_firestore_database.this.uid
}
