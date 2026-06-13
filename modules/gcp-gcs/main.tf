resource "random_id" "suffix" {
  byte_length = 3
}

locals {
  bucket_name = "${var.name}-${random_id.suffix.hex}"
}

resource "google_storage_bucket" "this" {
  name                        = local.bucket_name
  location                    = var.location
  storage_class               = var.storage_class
  force_destroy               = var.force_destroy
  uniform_bucket_level_access = true
  public_access_prevention    = var.block_public_access ? "enforced" : "inherited"

  versioning {
    enabled = var.versioning
  }

  dynamic "lifecycle_rule" {
    for_each = var.archive_after_days > 0 ? [1] : []
    content {
      condition {
        age = var.archive_after_days
      }
      action {
        type          = "SetStorageClass"
        storage_class = "ARCHIVE"
      }
    }
  }

  labels = var.labels
}
