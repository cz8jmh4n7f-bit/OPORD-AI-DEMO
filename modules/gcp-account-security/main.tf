variable "project_id" {
  type = string
}

variable "project_number" {
  type        = string
  default     = ""
  description = "Unused - the Cloud Storage service agent is now resolved via a data source (see google_storage_project_service_account below). Kept optional for backward-compat with the orchestrator's provision call; destroy doesn't pass it, so it MUST have a default or tofu destroy fails on a required var."
}

variable "csa_id" {
  type = string
}

variable "location" {
  type        = string
  default     = "europe"
  description = "Multi-region for the KMS keyring + log bucket."
}

variable "key_rotation_days" {
  type    = number
  default = 90
}

variable "log_retention_days" {
  type    = number
  default = 30
}

variable "labels" {
  type    = map(string)
  default = {}
}

locals {
  bucket_name = "${var.project_id}-logs-sink" # project_id is globally unique to bucket is too
  # KMS multi-region names ("europe"/"us"/"asia") differ from GCS ("EU"/"US"/"ASIA").
  # Map so the bucket gets a GCS-valid location; a regional value passes through
  # unchanged. (A GCS "EU" bucket is CMEK-compatible with a KMS "europe" key.)
  gcs_location = lookup({ europe = "EU", us = "US", asia = "ASIA" }, lower(var.location), var.location)
}

# The Cloud Storage service agent is created lazily - it does NOT exist in a
# brand-new project. This data source fetches it (the GET triggers creation),
# so the CMEK IAM binding below doesn't fail with "service account does not exist".
data "google_storage_project_service_account" "gcs" {
  project = var.project_id
}

resource "google_kms_key_ring" "logs" {
  name     = "opord-${var.csa_id}-logs-ring"
  project  = var.project_id
  location = var.location
}

resource "google_kms_crypto_key" "logs" {
  name            = "opord-${var.csa_id}-logs-key"
  key_ring        = google_kms_key_ring.logs.id
  rotation_period = "${var.key_rotation_days * 86400}s"
  purpose         = "ENCRYPT_DECRYPT"

  lifecycle {
    prevent_destroy = false
  }
}

# The Cloud Storage service agent must be able to use the key (CMEK).
resource "google_kms_crypto_key_iam_member" "storage" {
  crypto_key_id = google_kms_crypto_key.logs.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${data.google_storage_project_service_account.gcs.email_address}"
}

resource "google_storage_bucket" "logs" {
  name                        = local.bucket_name
  project                     = var.project_id
  location                    = local.gcs_location
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = true
  labels                      = var.labels

  encryption {
    default_kms_key_name = google_kms_crypto_key.logs.id
  }

  lifecycle_rule {
    condition {
      age = var.log_retention_days
    }
    action {
      type = "Delete"
    }
  }

  depends_on = [google_kms_crypto_key_iam_member.storage]
}

# Project log sink to the CMEK bucket. unique_writer_identity so we can grant it.
resource "google_logging_project_sink" "this" {
  name                   = "opord-${var.csa_id}-sink"
  project                = var.project_id
  destination            = "storage.googleapis.com/${google_storage_bucket.logs.name}"
  filter                 = "severity >= INFO"
  unique_writer_identity = true
}

resource "google_storage_bucket_iam_member" "sink_writer" {
  bucket = google_storage_bucket.logs.name
  role   = "roles/storage.objectCreator"
  member = google_logging_project_sink.this.writer_identity
}

# Log-based metrics for critical events (IAM policy changes, project deletion).
resource "google_logging_metric" "iam_changes" {
  name    = "opord_${var.csa_id}_iam_changes"
  project = var.project_id
  filter  = "protoPayload.methodName=\"SetIamPolicy\""
  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
  }
}

resource "google_logging_metric" "project_deletion" {
  name    = "opord_${var.csa_id}_project_deletion"
  project = var.project_id
  filter  = "protoPayload.methodName=\"DeleteProject\" OR protoPayload.methodName:\"projects.delete\""
  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
  }
}

output "kms_key_id" {
  value = google_kms_crypto_key.logs.id
}

output "log_bucket" {
  value = google_storage_bucket.logs.name
}

output "log_sink_writer" {
  value = google_logging_project_sink.this.writer_identity
}
