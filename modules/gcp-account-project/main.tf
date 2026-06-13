locals {
  # Project id: {csa_id}-{cloud_name}[suffix], lowercase, 6-30 chars, globally
  # unique + reserved 30 days after delete (handle naming with care).
  project_id = lower("${var.csa_id}-${var.cloud_name}${var.project_id_suffix}")

  # GCP label values must be [a-z0-9_-], <=63 chars. Sanitize the owner.
  owner_label = var.owner == "" ? "unset" : substr(replace(lower(var.owner), "/[^a-z0-9_-]/", "_"), 0, 63)

  labels = merge({
    csa_id      = var.csa_id
    environment = var.cloud_name
    owner       = local.owner_label
    managed_by  = var.managed_by
    cost_center = var.cost_center == "" ? var.csa_id : var.cost_center
  }, var.extra_labels)
}

# A folder per CSA under the project-factory parent (org isolation; display_name
# = the project id for at-a-glance correlation in the console).
resource "google_folder" "this" {
  display_name = local.project_id
  parent       = var.folder_parent
}

resource "google_project" "this" {
  name            = substr("${var.csa_id} ${var.cloud_name}", 0, 30)
  project_id      = local.project_id
  folder_id       = google_folder.this.folder_id
  billing_account = var.billing_account
  labels          = local.labels

  # CRITICAL: no default network (a default network has overly-permissive
  # firewall rules - the secure VPC layer provisions a locked one instead).
  auto_create_network = false

  deletion_policy = var.deletion_policy
}
