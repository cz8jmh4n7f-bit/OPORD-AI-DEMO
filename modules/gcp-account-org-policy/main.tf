variable "project_id" {
  type = string
}

variable "enforce_skip_default_network" {
  type    = bool
  default = true
}

variable "enforce_uniform_bucket_access" {
  type    = bool
  default = true
}

variable "enforce_public_access_prevention" {
  type    = bool
  default = true
}

variable "enforce_shielded_vm" {
  type        = bool
  default     = false
  description = "Require Shielded VMs. Default false (opt-in / audit first - can break existing images)."
}

variable "enforce_disable_sa_key_creation" {
  type        = bool
  default     = false
  description = "Block SA key creation (compliance). Default false - WIF is the preferred alternative."
}

variable "allowed_locations" {
  type        = list(string)
  default     = []
  description = "gcp.resourceLocations allowed values (e.g. ['in:europe-locations'] or specific regions). Empty to not applied."
}

variable "allowed_member_domains" {
  type        = list(string)
  default     = []
  description = "iam.allowedPolicyMemberDomains: DIRECTORY CUSTOMER IDS (not domain strings). Empty to not applied."
}

locals {
  bool_policies = {
    "compute.skipDefaultNetworkCreation"   = var.enforce_skip_default_network
    "storage.uniformBucketLevelAccess"     = var.enforce_uniform_bucket_access
    "storage.publicAccessPrevention"       = var.enforce_public_access_prevention
    "compute.requireShieldedVm"            = var.enforce_shielded_vm
    "iam.disableServiceAccountKeyCreation" = var.enforce_disable_sa_key_creation
  }
  enforced = { for k, v in local.bool_policies : k => v if v }
}

resource "google_org_policy_policy" "bool" {
  for_each = local.enforced
  name     = "projects/${var.project_id}/policies/${each.key}"
  parent   = "projects/${var.project_id}"

  spec {
    rules {
      enforce = "TRUE"
    }
  }
}

resource "google_org_policy_policy" "allowed_locations" {
  count  = length(var.allowed_locations) > 0 ? 1 : 0
  name   = "projects/${var.project_id}/policies/gcp.resourceLocations"
  parent = "projects/${var.project_id}"

  spec {
    rules {
      values {
        allowed_values = var.allowed_locations
      }
    }
  }
}

resource "google_org_policy_policy" "allowed_member_domains" {
  count  = length(var.allowed_member_domains) > 0 ? 1 : 0
  name   = "projects/${var.project_id}/policies/iam.allowedPolicyMemberDomains"
  parent = "projects/${var.project_id}"

  spec {
    rules {
      values {
        allowed_values = var.allowed_member_domains
      }
    }
  }
}

output "enforced_policies" {
  value = sort(keys(local.enforced))
}
