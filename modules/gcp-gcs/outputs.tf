output "bucket_id" {
  value       = google_storage_bucket.this.name
  description = "The bucket name."
}

output "bucket_arn" {
  value       = google_storage_bucket.this.self_link
  description = "GCS has no ARN; the API self_link is the closest stable identifier."
}

output "bucket_regional_domain_name" {
  value       = google_storage_bucket.this.url
  description = "The gs:// URL of the bucket."
}
