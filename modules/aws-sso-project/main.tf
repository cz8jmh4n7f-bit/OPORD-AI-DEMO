# Resolve the Identity Center instance + identity store. If the caller didn't
# pin them, use the org's single instance (the common case).
data "aws_ssoadmin_instances" "this" {}

locals {
  sso_arns   = tolist(data.aws_ssoadmin_instances.this.arns)
  sso_stores = tolist(data.aws_ssoadmin_instances.this.identity_store_ids)
  # Fall back to "" (instead of indexing an empty list) so the locals never throw
  # a cryptic "Invalid index" - the precondition below gives a clear message.
  has_instance = length(local.sso_arns) > 0

  instance_arn      = var.sso_instance_arn != "" ? var.sso_instance_arn : (local.has_instance ? local.sso_arns[0] : "")
  identity_store_id = var.identity_store_id != "" ? var.identity_store_id : (local.has_instance ? local.sso_stores[0] : "")

  create_permission_set = var.permission_set_name != ""
}

# --- The project group (group-per-project) ---
resource "aws_identitystore_group" "project" {
  identity_store_id = local.identity_store_id
  display_name      = "${var.group_prefix}${var.project_name}"
  description       = "OPORD-managed access group for project ${var.project_name}"

  lifecycle {
    precondition {
      condition     = var.sso_instance_arn != "" || local.has_instance
      error_message = "No IAM Identity Center instance found in this account/region. Enable Identity Center in your AWS Organization (or set sso_instance_arn), and run OPORD from the Identity Center management/delegated-admin account."
    }
  }
}

# Look up each requested user by username (they must already exist in Identity
# Center - OPORD assigns existing identities, it does not create them).
data "aws_identitystore_user" "members" {
  for_each          = toset(var.user_names)
  identity_store_id = local.identity_store_id

  alternate_identifier {
    unique_attribute {
      attribute_path  = "UserName"
      attribute_value = each.value
    }
  }
}

resource "aws_identitystore_group_membership" "members" {
  for_each          = data.aws_identitystore_user.members
  identity_store_id = local.identity_store_id
  group_id          = aws_identitystore_group.project.group_id
  member_id         = each.value.user_id
}

# --- Permission set: create one (with managed policies) or reference existing ---
resource "aws_ssoadmin_permission_set" "this" {
  count            = local.create_permission_set ? 1 : 0
  name             = var.permission_set_name
  description      = "OPORD-managed permission set for project ${var.project_name}"
  instance_arn     = local.instance_arn
  session_duration = var.session_duration
}

resource "aws_ssoadmin_managed_policy_attachment" "this" {
  for_each           = local.create_permission_set ? toset(var.managed_policy_arns) : toset([])
  instance_arn       = local.instance_arn
  permission_set_arn = aws_ssoadmin_permission_set.this[0].arn
  managed_policy_arn = each.value
}

locals {
  permission_set_arn = local.create_permission_set ? aws_ssoadmin_permission_set.this[0].arn : var.existing_permission_set_arn
}

# --- The grant: GROUP -> permission set -> target account ---
resource "aws_ssoadmin_account_assignment" "this" {
  instance_arn       = local.instance_arn
  permission_set_arn = local.permission_set_arn
  principal_id       = aws_identitystore_group.project.group_id
  principal_type     = "GROUP"
  target_id          = var.account_id
  target_type        = "AWS_ACCOUNT"

  # The managed-policy attachments shape what the permission set grants; make
  # sure they exist before the assignment provisions the role into the account.
  depends_on = [aws_ssoadmin_managed_policy_attachment.this]

  # Provisioning the permission set into the account (creating the reserved SSO
  # role + propagating) can take several minutes; the provider's short default
  # create-timeout expires first (Finding H - same class as the Security Hub
  # standards-subscription timeout), and the tainted resource then churns on
  # retry. A generous timeout lets the assignment settle on the first attempt.
  timeouts {
    create = "20m"
    delete = "20m"
  }
}
