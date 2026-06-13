# `gcp-account-iam` - project factory Layer: iam

Layer 6 of the GCP project factory (ADR-0011). Custom role + IAM bindings for
**existing** members (least privilege).

- **Custom role** `custom1_{csa_id}` with `custom_role_permissions` (a minimal
  read-ish default; override per workload).
- **Bindings** - a list of `{member, role}` applied via `google_project_iam_member`
  (additive, non-authoritative). The Go provider maps the spec role
  (Admin/Manager/Custom1) onto **non-primitive predefined roles** before calling
  this module.
- **Guardrail** - a `terraform_data` precondition **fails the plan** if a
  `user:`/`group:` member is granted `roles/owner` or `roles/editor` (primitive
  roles for humans are an anti-pattern).

## Scope note

This layer binds roles to members that **already exist** (in Cloud Identity /
Google Workspace). **Creating** Workspace users (with Azure AD name lookup +
Domain-Wide Delegation) is a separate, heavier opt-in layer (not v1) - DWD
cannot be fully configured via tofu (a manual Workspace Admin Console step).

## Inputs

`project_id`, `csa_id`, `custom_role_permissions`, `bindings` (list of
`{member, role}`).

## Outputs

`custom_role` (resource name), `binding_count`.
