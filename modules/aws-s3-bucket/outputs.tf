output "bucket_id" {
  description = "Bucket name (id)."
  value       = aws_s3_bucket.this.id
}

output "bucket_arn" {
  description = "Bucket ARN."
  value       = aws_s3_bucket.this.arn
}

output "bucket_regional_domain_name" {
  description = "Regional virtual-hosted domain (use as CloudFront origin)."
  value       = aws_s3_bucket.this.bucket_regional_domain_name
}
