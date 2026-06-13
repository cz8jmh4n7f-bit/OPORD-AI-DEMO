variable "region" {
  type        = string
  description = "AWS region."
}

variable "bucket_name" {
  type        = string
  description = "Globally-unique S3 bucket name."
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}

resource "aws_s3_bucket" "this" {
  bucket = var.bucket_name

  tags = {
    Environment = var.environment
    ManagedBy   = "opord"
  }
}

resource "aws_s3_bucket_public_access_block" "this" {
  bucket = aws_s3_bucket.this.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

output "bucket_id" {
  description = "The bucket name."
  value       = aws_s3_bucket.this.id
}

output "bucket_arn" {
  description = "The bucket ARN."
  value       = aws_s3_bucket.this.arn
}
