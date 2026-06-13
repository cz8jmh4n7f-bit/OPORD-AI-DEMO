# `gcp-account-project` - project factory Layer: project

Creates the foundation of a managed GCP project: a **per-CSA folder**
under the project-factory parent and a **GCP project** with billing linked, cost
labels, and **no default network**.

This is the first layer the OPORD GCP `AccountProvisioner` runs (ADR-0011). Its
`project_id` output feeds every downstream layer (apis, security, vpc, org-policy,
iam) via outputs - never data sources.

## Inputs (key)

| var | required | notes |
|---|---|---|
| `csa_id` | ✓ | unique CSA id; lowercase, drives the project id |
| `cloud_name` | ✓ | prod/stage/dev |
| `folder_parent` | ✓ | `organizations/NNN` or `folders/NNN` (from the provider config / OpenBao) |
| `billing_account` | ✓ | `XXXXXX-XXXXXX-XXXXXX` (from the provider config / OpenBao) |
| `project_id_suffix` | | optional, for global-uniqueness collision avoidance |
| `deletion_policy` | | `DELETE` (dev) / `PREVENT` (prod) |

`project_id = {csa_id}-{cloud_name}{suffix}`.

## ⚠️ GCP gotchas this layer handles / you must know

- **`auto_create_network = false`** - the default network ships permissive
  firewall rules; the `vpc` layer creates a locked one instead. Never `true`.
- **Project id is globally unique + reserved 30 days after delete.** Once
  `{csa_id}-{cloud_name}` is used you cannot reuse that exact id for 30 days even
  after `tofu destroy`. Pick `csa_id` carefully; use `project_id_suffix` if you
  need to recreate sooner. **Test on a dev organization.**
- **Billing propagation** - after the project links a billing account it can take
  1-2 min before billing-dependent APIs/resources work. The `apis` layer's
  `time_sleep` covers part of this.
- **`deletion_policy`** - newer `google` providers default to `PREVENT` (destroy
  blocked). This module defaults to `DELETE` for dev; set `PREVENT` for prod
  the project factory so a project is never destroyed by accident.

## Outputs

`project_id`, `project_number`, `folder_id`, `folder_name`, `labels`.

## Provisioning identity

Runs as the org-level provisioning SA (creds from OpenBao via the GCP provider).
That identity needs, at the folder/org level:
`roles/resourcemanager.folderAdmin`, `roles/resourcemanager.projectCreator`,
`roles/billing.user`. See `docs/` (org-setup runbook, a later iteration).
