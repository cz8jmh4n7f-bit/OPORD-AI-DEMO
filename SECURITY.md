# Security Policy

OPORD is a self-hosted infrastructure & AI governance platform. Security issues
are taken seriously - thank you for reporting them responsibly.

## Reporting a vulnerability

- Email **security@opord.dev** with a description, reproduction steps, and impact.
- Please do **not** open a public issue for security reports.
- You should receive an acknowledgement within 72 hours. Coordinated disclosure
  is appreciated; we will credit reporters unless you prefer otherwise.

## Scope

- The OPORD control plane (API, worker, CLI) and web console in this repository.
- The bundled deployment manifests under `deployments/`.

Out of scope: vulnerabilities in third-party backends OPORD orchestrates
(Proxmox, vSphere, AWS, Azure, GCP, OpenAI, Anthropic) - report those upstream.

## Product security model (summary)

- **Secrets by reference.** Provider and AI keys live in your Vault/OpenBao and
  resolve per request via `secret_ref`; OPORD never stores raw keys in its
  database and redacts secret-shaped config in API responses.
- **AuthN/AuthZ.** API-key RBAC (`viewer` < `operator` < `admin`) with hashed
  keys; tenant data-scoping on reads and writes.
- **Audit.** Requests, approvals, grants, revokes, and governance blocks are
  durably recorded with actor and timestamp (CSV export in the console).
- **Web console.** The browser talks to a same-origin proxy that holds the
  session key in an HttpOnly cookie; a nonce-based CSP and baseline security
  headers are applied to every response.
- **Encryption.** Optional OpenTofu state encryption at rest (AES-GCM); run the
  stack behind your own ingress/TLS.

## Supported versions

Alpha: only the latest `main` is supported. Update before reporting where
practical.
