variable "project_id" {
  type = string
}

variable "csa_id" {
  type = string
}

variable "custom_role_permissions" {
  type        = list(string)
  description = "Permissions for the project custom1 role (least privilege)."
  default = [
    "resourcemanager.projects.get",
    "compute.instances.list",
    "compute.instances.get",
    "storage.buckets.list",
    "storage.objects.list",
    "logging.logEntries.list",
    "monitoring.timeSeries.list",
  ]
}

variable "bindings" {
  type = list(object({
    member = string # "user:a@b.com" / "group:g@b.com" / "serviceAccount:..."
    role   = string # predefined or the custom role id; NEVER roles/owner|roles/editor for humans
  }))
  default     = []
  description = "IAM role bindings for existing members. The Go provider maps the spec role (Admin/Manager/Custom1) onto non-primitive predefined roles."
}

locals {
  custom_role_id = "custom1_${replace(var.csa_id, "-", "_")}"
  # Reject primitive roles for humans (user:/group:) at plan time.
  primitive_for_humans = [
    for b in var.bindings : b
    if(startswith(b.member, "user:") || startswith(b.member, "group:")) &&
    contains(["roles/owner", "roles/editor"], b.role)
  ]
}

resource "google_project_iam_custom_role" "custom1" {
  count       = length(var.custom_role_permissions) > 0 ? 1 : 0
  role_id     = local.custom_role_id
  project     = var.project_id
  title       = "OPORD Custom1 ${var.csa_id}"
  description = "Least-privilege custom role for ${var.csa_id} (OPORD)."
  permissions = var.custom_role_permissions
}

# Fail the plan if a human is being granted a primitive role (anti-pattern).
resource "terraform_data" "no_primitive_for_humans" {
  lifecycle {
    precondition {
      condition     = length(local.primitive_for_humans) == 0
      error_message = "Primitive roles (roles/owner, roles/editor) must not be granted to user:/group: members. Use predefined or the custom role."
    }
  }
}

resource "google_project_iam_member" "bindings" {
  for_each = { for b in var.bindings : "${b.member}|${b.role}" => b }
  project  = var.project_id
  role     = each.value.role
  member   = each.value.member
}

variable "project_number" {
  type        = string
  default     = ""
  description = "Project number. When set, grants the Compute Engine default SA (the 2nd-gen Cloud Functions build agent) the roles a function build needs, so a Cloud Function deploys into a freshly-created governed project without failing on a missing build-SA permission."
}

# 2nd-gen Cloud Functions build runs as the Compute Engine default SA; a fresh
# governed project's build agent has no roles, so a function build fails. Grant the
# minimal set so the catalog's Function card works when deployed into a new project.
resource "google_project_iam_member" "build_agent" {
  for_each = var.project_number == "" ? toset([]) : toset([
    "roles/cloudbuild.builds.builder",
    "roles/artifactregistry.writer",
    "roles/logging.logWriter",
  ])
  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${var.project_number}-compute@developer.gserviceaccount.com"
}

output "custom_role" {
  value = length(google_project_iam_custom_role.custom1) > 0 ? google_project_iam_custom_role.custom1[0].name : ""
}

output "binding_count" {
  value = length(var.bindings)
}
