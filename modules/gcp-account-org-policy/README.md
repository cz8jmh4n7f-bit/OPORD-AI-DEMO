# `gcp-account-org-policy` - project factory Layer: org-policy

Layer 5 of the GCP project factory (ADR-0011). Project-level Organization Policy
constraints (v2 `orgpolicy.googleapis.com`).

## Boolean guardrails (enforced by default)

| constraint | default | why |
|---|---|---|
| `compute.skipDefaultNetworkCreation` | ✓ | extra belt over `auto_create_network=false` |
| `storage.uniformBucketLevelAccess` | ✓ | no per-object ACLs |
| `storage.publicAccessPrevention` | ✓ | no public buckets |
| `compute.requireShieldedVm` | ✗ | opt-in (can break non-Shielded images - audit first) |
| `iam.disableServiceAccountKeyCreation` | ✗ | opt-in (WIF is the preferred alternative) |

## List constraints (applied only when provided)

- `gcp.resourceLocations` `allowed_locations` (e.g. `["in:europe-locations"]`).
- `iam.allowedPolicyMemberDomains` `allowed_member_domains` - **directory
  CUSTOMER IDS**, not domain strings (a common gotcha).

## Gotchas

- Some constraints (e.g. `compute.skipDefaultNetworkCreation`) are best set at the
  **organization** level, not per-project - this layer sets the project-level
  override; the org-level baseline is an org-setup prerequisite (runbook).
- Org Policy changes can take 1-2 min to take effect (eventual consistency).

## Outputs

`enforced_policies` (sorted list of enforced boolean constraints).
