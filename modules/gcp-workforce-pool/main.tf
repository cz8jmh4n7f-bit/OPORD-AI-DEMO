# OPORD GCP Workforce Identity Federation pool (ADR-0012) - one-time, org-level.
# Federates Microsoft Entra ID into GCP so workforce users sign in to the Console /
# gcloud with their Entra credentials (NO Google accounts / no provisioning). The
# `project` primitive then grants project IAM to
#   principalSet://iam.googleapis.com/locations/global/workforcePools/<pool>/group/<entra-group-id>

variable "org_id" {
  type        = string
  description = "Numeric organization id that owns the workforce pool."
}

variable "pool_id" {
  type        = string
  default     = "opord-entra"
  description = "Workforce identity pool id (4-32 chars; start with a letter; lowercase/digits/hyphens)."
  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{3,31}$", var.pool_id))
    error_message = "pool_id must be 4-32 chars, start with a letter, and use lowercase letters/digits/hyphens."
  }
}

variable "pool_display_name" {
  type    = string
  default = "OPORD Entra workforce pool"
}

variable "provider_id" {
  type        = string
  default     = "entra-oidc"
  description = "Workforce pool provider id."
}

variable "entra_tenant_id" {
  type        = string
  description = "Microsoft Entra tenant id (drives the OIDC issuer)."
}

variable "entra_client_id" {
  type        = string
  description = "Application (client) id of the Entra app registration used for WIF."
}

variable "entra_client_secret" {
  type        = string
  sensitive   = true
  description = "Client secret of the Entra app - required for the CODE web-SSO flow."
}

variable "session_duration" {
  type        = string
  default     = "3600s"
  description = "Console/gcloud session lifetime for federated users."
}

resource "google_iam_workforce_pool" "this" {
  parent            = "organizations/${var.org_id}"
  location          = "global"
  workforce_pool_id = var.pool_id
  display_name      = var.pool_display_name
  description       = "OPORD-managed workforce pool federating Microsoft Entra ID into GCP."
  session_duration  = var.session_duration
}

resource "google_iam_workforce_pool_provider" "entra" {
  location          = "global"
  workforce_pool_id = google_iam_workforce_pool.this.workforce_pool_id
  provider_id       = var.provider_id
  display_name      = "Microsoft Entra ID (OIDC)"
  description       = "Entra OIDC provider for OPORD workforce federation."

  # Map Entra token claims onto Google attributes. google.groups carries the Entra
  # group object ids so project IAM can target principalSet .../group/<group-id>.
  attribute_mapping = {
    "google.subject"      = "assertion.sub"
    "google.display_name" = "assertion.preferred_username"
    "google.groups"       = "assertion.groups"
  }

  oidc {
    issuer_uri = "https://login.microsoftonline.com/${var.entra_tenant_id}/v2.0"
    client_id  = var.entra_client_id
    client_secret {
      value {
        plain_text = var.entra_client_secret
      }
    }
    web_sso_config {
      response_type             = "CODE"
      assertion_claims_behavior = "MERGE_USER_INFO_OVER_ID_TOKEN_CLAIMS"
    }
  }
}

output "workforce_pool_id" {
  value = google_iam_workforce_pool.this.workforce_pool_id
}

output "workforce_pool_resource" {
  value = google_iam_workforce_pool.this.name
}

output "provider_id" {
  value = google_iam_workforce_pool_provider.entra.provider_id
}

output "principal_set_prefix" {
  value       = "principalSet://iam.googleapis.com/locations/global/workforcePools/${google_iam_workforce_pool.this.workforce_pool_id}"
  description = "Prefix for project IAM members; append /group/<entra-group-id> for a group, or /* for all pool users."
}
