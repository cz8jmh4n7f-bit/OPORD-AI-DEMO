output "project_id" {
  value       = google_project.this.project_id
  description = "The created project id (consumed by every downstream layer)."
}

output "project_number" {
  value       = google_project.this.number
  description = "The project number (used by service-agent IAM bindings)."
}

output "folder_id" {
  value       = google_folder.this.folder_id
  description = "The per-CSA folder id (numeric)."
}

output "folder_name" {
  value       = google_folder.this.name
  description = "The per-CSA folder resource name (folders/NNN)."
}

output "labels" {
  value = local.labels
}
