resource "aws_organizations_account" "this" {
  name      = var.account_name
  email     = var.email
  parent_id = var.ou_id != "" ? var.ou_id : null
  role_name = var.access_role_name

  # Members can see their own billing (needed for cost tooling / FinOps).
  iam_user_access_to_billing = "ALLOW"

  # Safety: by default DON'T close the account on destroy (closure is a 90-day,
  # irreversible operation gated behind an explicit OPORD decommission action).
  close_on_deletion = var.close_on_deletion

  tags = var.tags

  lifecycle {
    # Re-running must not churn the account if only the bootstrap role name
    # drifts (it can't be changed after creation anyway).
    ignore_changes = [role_name]
  }
}
