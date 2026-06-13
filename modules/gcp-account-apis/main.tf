variable "project_id" {
  type        = string
  description = "Project to enable services on (from the project layer's output)."
}

variable "apis" {
  type        = list(string)
  description = "Base required APIs."
  default = [
    "cloudresourcemanager.googleapis.com",
    "cloudbilling.googleapis.com",
    "serviceusage.googleapis.com",
    "iam.googleapis.com",
    "cloudkms.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
    "storage.googleapis.com",
  ]
}

variable "create_vpc" {
  type        = bool
  default     = true
  description = "When true, also enable compute.googleapis.com (needed by the secure-vpc layer)."
}

variable "enable_org_policy" {
  type        = bool
  default     = true
  description = "When true, also enable orgpolicy.googleapis.com (org-policy layer)."
}

variable "extra_apis" {
  type    = list(string)
  default = []
}

variable "enable_catalog_apis" {
  type        = bool
  default     = true
  description = "When true, enable the service APIs the OPORD catalog deploys into a project (Secret Manager, Pub/Sub, Memorystore, Cloud SQL, Cloud Functions, Firestore, GKE) so catalog items deploy into a freshly-created project without a manual `gcloud services enable`."
}

variable "catalog_apis" {
  type = list(string)
  default = [
    "secretmanager.googleapis.com", # Secret
    "pubsub.googleapis.com",        # Queue
    "redis.googleapis.com",         # Cache (Memorystore)
    "sqladmin.googleapis.com",      # Database (Cloud SQL)
    "cloudfunctions.googleapis.com", "run.googleapis.com", "cloudbuild.googleapis.com", "artifactregistry.googleapis.com", # Function (2nd-gen)
    "firestore.googleapis.com", # Table
    "container.googleapis.com", # Kubernetes (GKE)
  ]
}

variable "wait_seconds" {
  type        = number
  default     = 60
  description = "Seconds to wait after enabling for API propagation (30-60s typical)."
}

locals {
  apis = toset(concat(
    var.apis,
    var.create_vpc ? ["compute.googleapis.com"] : [],
    var.enable_org_policy ? ["orgpolicy.googleapis.com"] : [],
    var.enable_catalog_apis ? var.catalog_apis : [],
    var.extra_apis,
  ))
}

resource "google_project_service" "this" {
  for_each = local.apis
  project  = var.project_id
  service  = each.value

  # Keep dependencies + the service enabled on destroy (avoid breaking other
  # resources / API churn). The project's own deletion handles teardown.
  disable_dependent_services = false
  disable_on_destroy         = false
}

# API enablement is eventually consistent - give it time before downstream layers
# create resources that depend on these APIs.
resource "time_sleep" "propagate" {
  depends_on      = [google_project_service.this]
  create_duration = "${var.wait_seconds}s"
}

output "enabled_apis" {
  value = sort(tolist(local.apis))
}

output "ready" {
  value       = time_sleep.propagate.id
  description = "Propagation-complete signal; downstream layers run after this layer (provider sequencing)."
}
