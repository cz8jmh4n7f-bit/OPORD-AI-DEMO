# Native OpenTofu tests for the gcp-account-project layer (offline, mock_provider).
#   cd modules/gcp-account-project && tofu test

mock_provider "google" {}

variables {
  csa_id          = "acme"
  cloud_name      = "dev"
  folder_parent   = "folders/123"
  billing_account = "XXXXXX-XXXXXX-XXXXXX"
  owner           = "alice@example.com"
}

run "project_id_and_security" {
  command = plan

  assert {
    condition     = google_project.this.project_id == "acme-dev"
    error_message = "project_id should be {csa_id}-{cloud_name}"
  }
  assert {
    condition     = google_project.this.auto_create_network == false
    error_message = "auto_create_network MUST be false (default network is insecure)"
  }
  assert {
    condition     = google_project.this.labels["csa_id"] == "acme" && google_project.this.labels["environment"] == "dev"
    error_message = "cost-allocation labels missing"
  }
  assert {
    condition     = google_folder.this.display_name == "acme-dev"
    error_message = "per-CSA folder display_name should be the project id"
  }
}

run "project_id_suffix" {
  command = plan
  variables {
    project_id_suffix = "-x1"
  }
  assert {
    condition     = google_project.this.project_id == "acme-dev-x1"
    error_message = "project_id_suffix should append to the project id"
  }
}

run "rejects_bad_csa_id" {
  command = plan
  variables {
    csa_id = "Bad_ID"
  }
  expect_failures = [var.csa_id]
}
