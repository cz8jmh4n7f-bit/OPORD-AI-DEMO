locals {
  base_tags = merge(var.tags, { Name = var.name, ManagedBy = "opord" })
}

resource "aws_s3_bucket" "this" {
  bucket = var.name
  tags   = local.base_tags
}

# Block-public-access: 4 protections. Off only by explicit opt-in.
resource "aws_s3_bucket_public_access_block" "this" {
  count                   = var.block_public_access ? 1 : 0
  bucket                  = aws_s3_bucket.this.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "this" {
  bucket = aws_s3_bucket.this.id
  versioning_configuration {
    status = var.versioning ? "Enabled" : "Suspended"
  }
}

# Encryption at rest. SSE-KMS when kms_key_arn given (charges per request),
# otherwise SSE-S3 (free, AES256) which still encrypts every object.
resource "aws_s3_bucket_server_side_encryption_configuration" "this" {
  bucket = aws_s3_bucket.this.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = var.kms_key_arn == "" ? "AES256" : "aws:kms"
      kms_master_key_id = var.kms_key_arn == "" ? null : var.kms_key_arn
    }
    bucket_key_enabled = var.kms_key_arn != ""
  }
}

# Optional lifecycle: archive cold objects to Glacier Deep Archive.
resource "aws_s3_bucket_lifecycle_configuration" "this" {
  count  = var.lifecycle_glacier_days > 0 ? 1 : 0
  bucket = aws_s3_bucket.this.id
  rule {
    id     = "opord-glacier-tier"
    status = "Enabled"
    filter {}
    transition {
      days          = var.lifecycle_glacier_days
      storage_class = "DEEP_ARCHIVE"
    }
  }
}
