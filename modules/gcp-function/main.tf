variable "name" {
  type = string
}

variable "region" {
  type    = string
  default = "europe-west1"
}

variable "runtime" {
  type    = string
  default = "python312"
}

variable "entry_point" {
  type    = string
  default = "hello"
}

variable "memory_mb" {
  type    = number
  default = 256
}

variable "timeout_sec" {
  type    = number
  default = 60
}

variable "env_vars" {
  type    = map(string)
  default = {}
}

# Optional external source (an existing GCS object). When unset, the built-in
# hello handler is zipped + uploaded.
variable "source_bucket" {
  type    = string
  default = ""
}

variable "source_object" {
  type    = string
  default = ""
}

variable "labels" {
  type    = map(string)
  default = {}
}

locals {
  use_builtin = var.source_bucket == "" || var.source_object == ""
}

resource "random_id" "suffix" {
  byte_length = 3
}

# --- built-in hello source (only when no external source is given) ---

resource "google_storage_bucket" "src" {
  count                       = local.use_builtin ? 1 : 0
  name                        = "${var.name}-src-${random_id.suffix.hex}"
  location                    = var.region
  uniform_bucket_level_access = true
  force_destroy               = true
  labels                      = var.labels
}

data "archive_file" "src" {
  count       = local.use_builtin ? 1 : 0
  type        = "zip"
  output_path = "${path.module}/.build/${var.name}.zip"

  source {
    filename = "main.py"
    content  = <<-PY
      import functions_framework

      @functions_framework.http
      def hello(request):
          return "Hello from OPORD on GCP Cloud Functions!\n"
    PY
  }

  source {
    filename = "requirements.txt"
    content  = "functions-framework==3.*\n"
  }
}

resource "google_storage_bucket_object" "src" {
  count  = local.use_builtin ? 1 : 0
  name   = "${var.name}-${data.archive_file.src[0].output_md5}.zip"
  bucket = google_storage_bucket.src[0].name
  source = data.archive_file.src[0].output_path
}

resource "google_cloudfunctions2_function" "this" {
  name     = var.name
  location = var.region
  labels   = var.labels

  build_config {
    runtime     = var.runtime
    entry_point = var.entry_point
    source {
      storage_source {
        bucket = local.use_builtin ? google_storage_bucket.src[0].name : var.source_bucket
        object = local.use_builtin ? google_storage_bucket_object.src[0].name : var.source_object
      }
    }
  }

  service_config {
    # Mi (mebibytes), not M (megabytes): Cloud Run 2nd-gen requires memory >= 128Mi
    # (134 MB). "128M" (128 MB = 122Mi) is BELOW that minimum and the deploy fails
    # ("memory must be between 128Mi and 512Mi inclusive"). Mi keeps 128 valid.
    available_memory      = "${var.memory_mb}Mi"
    timeout_seconds       = var.timeout_sec
    environment_variables = var.env_vars
  }
}

output "arn" {
  value       = google_cloudfunctions2_function.this.id
  description = "The function resource id (projects/.../functions/...)."
}

output "name" {
  value = google_cloudfunctions2_function.this.name
}

output "uri" {
  value       = google_cloudfunctions2_function.this.service_config[0].uri
  description = "The HTTPS endpoint of the underlying Cloud Run service."
}
