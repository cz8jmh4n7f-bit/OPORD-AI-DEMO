# `gcp-account-apis` - project factory Layer: apis

Enables the required project services and **waits for enablement to propagate**.
Layer 2 of the GCP project factory (ADR-0011); runs after `project`.

## Why the wait

GCP **API enablement is eventually consistent**: after `google_project_service`
reports a service enabled, creating a resource that uses that API can still fail
for ~30-60s. A `time_sleep` (`wait_seconds`, default 60) bridges this so the
downstream layers (security/vpc/org-policy/iam) don't hit "API not yet usable".

## Inputs

| var | default | notes |
|---|---|---|
| `project_id` | - | from the project layer output |
| `apis` | 8 base APIs | resourcemanager, billing, serviceusage, iam, kms, logging, monitoring, storage |
| `create_vpc` | `true` | adds `compute.googleapis.com` |
| `enable_org_policy` | `true` | adds `orgpolicy.googleapis.com` |
| `extra_apis` | `[]` | anything else the workload needs |
| `wait_seconds` | `60` | propagation wait |

## Gotchas handled

- **`disable_on_destroy = false`** - destroying this layer does NOT disable the
  APIs (avoids churn + breaking other resources). The project's own deletion is
  the real teardown.
- **`disable_dependent_services = false`** - never cascade-disable dependent APIs.
- **No `sleep N` in scripts** - the wait is a declarative `time_sleep` resource,
  state-tracked + idempotent (not a shell sleep).

## Outputs

`enabled_apis` (sorted list), `ready` (propagation-complete signal).
