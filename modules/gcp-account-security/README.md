# `gcp-account-security` - project factory Layer: security

Layer 3 of the GCP project factory (ADR-0011). Establishes the project's
encryption + audit foundation:

- **KMS keyring + crypto key** (`key_rotation_days`, default 90) - `ENCRYPT_DECRYPT`.
- **CMEK grant** - the Cloud Storage service agent gets
  `roles/cloudkms.cryptoKeyEncrypterDecrypter` so the bucket can use the key.
- **Log-sink bucket** - UBLA on, public-access-prevention enforced, **CMEK
  encrypted**, `log_retention_days` (default 30) lifecycle delete.
- **Project log sink** (`severity >= INFO`) to the bucket, with a unique writer
  identity granted `roles/storage.objectCreator`.
- **Log-based metrics** - `iam_changes` (SetIamPolicy) + `project_deletion`, for
  alerting on critical events.

## Inputs

`project_id`, `project_number` (CMEK service-agent grant), `csa_id`, `location`
(default `europe`), `key_rotation_days`, `log_retention_days`, `labels`.

## Gotchas handled

- **CMEK ordering** - the bucket `depends_on` the KMS IAM grant; without it the
  bucket create fails ("service account does not have permission to use the key").
- **Bucket global uniqueness** - name = `{project_id}-logs-sink`; project_id is
  globally unique, so the bucket is too.
- **`force_destroy = true`** is set for dev so `tofu destroy` removes a non-empty
  log bucket; prod should set it false (retain audit logs).

## Outputs

`kms_key_id`, `log_bucket`, `log_sink_writer`.
