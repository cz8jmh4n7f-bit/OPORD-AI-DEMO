# `gcp-workforce-pool` - Entra to GCP Workforce Identity Federation (ADR-0012)

One-time, **org-level** setup that federates **Microsoft Entra ID** into GCP via
**Workforce Identity Federation (WIF)**. Federated users sign in to the Google
Cloud Console / `gcloud` with their **Entra** credentials - no Google accounts, no
user provisioning. The `project` access primitive then grants project IAM to the
federated principals.

## What it creates

- `google_iam_workforce_pool` - the org-level pool (`organizations/<org_id>`).
- `google_iam_workforce_pool_provider` - an **OIDC** provider pointing at Entra
  (`https://login.microsoftonline.com/<tenant>/v2.0`), with an attribute mapping
  that carries `assertion.groups` to `google.groups` so project IAM can target an
  Entra **group**.

## How access is then granted (the `project` primitive)

Grant a project IAM role to an Entra group via the principal the pool exposes:

```
principalSet://iam.googleapis.com/locations/global/workforcePools/<pool>/group/<entra-group-object-id>
```

(or `…/group/<id>` per group, `…/subject/<sub>` per user, or `…/*` for all pool
users). OPORD's `project` primitive builds this string for you from the Entra
group id + the pool id.

## Operator prerequisites (one-time)

1. **Entra app registration** for the federation, configured to:
   - emit the **groups** claim (Token configuration to add groups claim to Group ID),
   - have a **client secret** (used by the CODE web-SSO flow),
   - redirect URI `https://auth.cloud.google/signin-callback` (GCP WIF callback).
2. GCP **org-level** permission to manage workforce pools
   (`roles/iam.workforcePoolAdmin` on the organization).
3. Store the Entra `tenant_id` / `client_id` / `client_secret` in OpenBao (OPORD
   reads them; never in the spec).

## Inputs

| var | required | notes |
|---|---|---|
| `org_id` | ✅ | numeric organization id |
| `entra_tenant_id` | ✅ | Entra tenant (OIDC issuer) |
| `entra_client_id` | ✅ | Entra app (client) id |
| `entra_client_secret` | ✅ (sensitive) | Entra app secret (CODE flow) |
| `pool_id` | - | default `opord-entra` |
| `provider_id` | - | default `entra-oidc` |
| `session_duration` | - | default `3600s` |

## Outputs

`workforce_pool_id`, `provider_id`, and `principal_set_prefix` (the
`principalSet://…/workforcePools/<pool>` prefix to append `/group/<id>` to).

## Notes

- WIF grants **access** (sign-in + IAM); it does **not** create Google identities.
  This is the lighter alternative to the "Google Cloud / G Suite Connector" gallery
  app (which federates into Cloud Identity/Workspace + SCIM-provisions users).
- Set up once per organization; the pool id then feeds the GCP provider config.
