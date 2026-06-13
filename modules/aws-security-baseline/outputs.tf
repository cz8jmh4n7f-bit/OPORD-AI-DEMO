output "cloudtrail_arn" {
  description = "ARN of the multi-region CloudTrail."
  value       = aws_cloudtrail.this.arn
}

output "cloudtrail_bucket" {
  description = "S3 bucket holding the trail logs."
  value       = aws_s3_bucket.trail.id
}

output "cloudtrail_kms_key_arn" {
  description = "KMS key encrypting the trail."
  value       = aws_kms_key.cloudtrail.arn
}

output "guardduty_detector_id" {
  value = aws_guardduty_detector.this.id
}

output "securityhub_enabled" {
  value = true
}
