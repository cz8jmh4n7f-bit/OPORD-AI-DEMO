variable "project_id" {
  type        = string
  description = "The GCP project to grant the role on."
}

variable "role" {
  type        = string
  description = "The IAM role to grant (e.g. roles/viewer)."
}

variable "members" {
  type        = list(string)
  default     = []
  description = "Members: bare emails (treated as user:) or prefixed user:/group:/serviceAccount:/domain:/principal:/principalSet: (the latter two are Workforce/Workload Identity Federation principals, e.g. Entra-via-WIF)."
}

variable "label" {
  type    = string
  default = ""
}

locals {
  # Members already carrying a recognized IAM prefix pass through unchanged; a bare
  # email is treated as user:. principalSet:// / principal:// are federated
  # (Workforce/Workload Identity Federation) principals - e.g. an Entra group via
  # WIF: principalSet://iam.googleapis.com/locations/global/workforcePools/<pool>/group/<id>.
  normalized = [for m in var.members : (
    can(regex("^(user|group|serviceAccount|domain|principalSet|principal):", m)) ? m : "user:${m}"
  )]
}

resource "google_project_iam_member" "this" {
  for_each = toset(local.normalized)
  project  = var.project_id
  role     = var.role
  member   = each.value
}

output "project_id" {
  value = var.project_id
}

output "role" {
  value = var.role
}

output "member_count" {
  value = length(local.normalized)
}

output "group_name" {
  value = var.label
}
