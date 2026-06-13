# OPORD — AI Service Governance

Self-host a **governance layer in front of your AI providers** (OpenAI, Anthropic, …).
Teams request access to AI services; you approve, meter, and audit it — with
**policies, seat quotas, and budgets enforced on every grant**, and a full audit
trail. Your provider keys stay in your own infrastructure.

> The name is the military term *Operation Order* (OPORD): the complete order a
> unit executes. Here, users issue declarative access **requests** and OPORD
> reconciles who can use which AI service.

This build is **AI-first**: the console opens straight into the AI workspace. (OPORD
is also a multi-cloud infrastructure platform — that surface ships in the codebase
but is gated behind the **AI** toggle; see [Infrastructure](#what-about-infrastructure).)

---

## Quick start (≈60 seconds)

Requires only **Docker** (Desktop or Engine + Compose v2).

```bash
git clone <this-repo> opord && cd opord
docker compose -f deployments/ai-compose.yml up --build
```

Open **<http://localhost:3000>**, click the neon **AI** sign, then **Enter the AI
workspace**. The catalog is seeded with **MockAI**, so it works with **no secrets**
out of the box — you can drive the whole request → approve → govern → audit flow
immediately.

- Web console: <http://localhost:3000>
- API: <http://localhost:8080> (auth disabled in this profile — see [Multi-user](#multi-user--authentication-optional))

Stack: `db` (Postgres) → `migrate` (schema + MockAI seed) → `api` → `worker` → `web`.

---

## Connect your own AI providers

This is the point of the product: govern access to **your** AI accounts. OPORD uses
**one org-level key per provider**. End users never receive raw keys — they receive
**governed access** (and, optionally, a metered [proxy](#proxy-real-usage--ai-gateway)).
**OPORD never stores the raw key in its database.** This stack bundles **OpenBao**
(a Vault-compatible secret store): you put the key there once and OPORD reads it
*by reference* (`secret_ref`), encrypted at rest. An environment variable works too,
as a quick alternative.

### Supported providers

| Provider | Add-provider type | Key (env var) | Status |
|----------|-------------------|---------------|--------|
| OpenAI / ChatGPT | `OpenAI / ChatGPT` | `OPENAI_API_KEY` | ✅ supported |
| Anthropic / Claude | `Anthropic / Claude Code` | `ANTHROPIC_API_KEY` | ✅ supported |
| MockAI (built-in demo) | `MockAI` | — | ✅ seeded |
| Gemini · GitHub Copilot · Cursor | — | — | declared, not yet implemented |

### Option A — secret store (recommended)

The stack bundles **OpenBao** (auto-unsealed, not exposed on the host). Store your key
once and reference it — encrypted at rest, never in OPORD's database or container env:

```bash
docker compose -f deployments/ai-compose.yml up -d            # start the stack
docker compose -f deployments/ai-compose.yml exec openbao \
  bao kv put secret/opord/ai/openai-main api_key=sk-...        # store the key
```

Then register the provider in the console:

1. **AI Providers** → **Add provider**.
2. **Type** = `OpenAI / ChatGPT` (or `Anthropic / Claude Code`); **Name** = `openai-main`.
3. **secret_ref** = `opord/ai/openai-main` (the path you stored, without the `secret/` mount prefix).
4. **Save** → **Check** (OPORD validates the key live against `/v1/models`) → **Sync** (imports the catalog).

Accepted keys in the KV entry: `api_key`, `openai_api_key`, `anthropic_api_key`, `token`.

> **Keep the key out of your shell history** — enter it interactively:
> ```bash
> docker compose -f deployments/ai-compose.yml exec openbao \
>   sh -c 'printf "key: "; read -r K; bao kv put secret/opord/ai/openai-main api_key="$K"'
> ```
> The bundled OpenBao is **dev-mode** (in-memory): re-add keys after a full `down`. For
> production, point `VAULT_ADDR` / `VAULT_TOKEN` at an external, persistent OpenBao or
> Vault (AppRole + TLS).

### Option B — environment variable (quick, no secret store)

Skip OpenBao and pass the key on `up` — simplest, but the key sits in your shell
history and the container's env:

```bash
OPENAI_API_KEY=sk-...  ANTHROPIC_API_KEY=sk-ant-...  \
  docker compose -f deployments/ai-compose.yml up -d
```

Then **Add provider** with the **`secret_ref` field left blank** — OPORD falls back to
the `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` env var. Then **Check** → **Sync**.

> Either way, the same via API:
> ```bash
> curl -X POST localhost:8080/api/v1/ai/providers -H 'Content-Type: application/json' \
>   -d '{"name":"openai-main","type":"openai","secretRef":"opord/ai/openai-main"}'  # omit secretRef to use env
> curl -X POST localhost:8080/api/v1/ai/providers/openai-main/check   # validate the key
> curl -X POST localhost:8080/api/v1/ai/providers/openai-main/sync    # import services
> ```

---

## Use it — the access workflow

1. **Browse AI services** (`/ai/catalog`) → **Request** governed access (owner,
   workspace, justification, optional expiry).
2. **Approve** (`/ai/requests`) → an operator approves or rejects. (Requests can be
   blocked here by [governance](#govern-it--enforcement); see below.)
3. On approval OPORD creates an **access instance** (`/ai/instances`) with owner,
   expiry, and provider access id. **Revoke** any time.
4. **Audit** (`/ai/audit`) → every request, approval, grant, revoke, **and block** is
   logged with actor and timestamp.

The **AI workspace** (`/ai/overview`) shows the live picture: providers, services,
active access, pending requests, and your governance posture.

---

## Govern it — enforcement

Every request is checked against your **active policies, quotas, and budgets** before
access is granted. A blocked request returns **HTTP 403 with a reason** and is
audited. Create these in the console or via the API.

### Policies — deny-list guardrails

A policy **denies** the requests it matches. Every non-empty selector must match
(AND); an empty selector is a wildcard.

```bash
curl -X POST localhost:8080/api/v1/ai/policies -H 'Content-Type: application/json' -d '{
  "name": "no-contractors-on-openai",
  "rules": { "effect": "deny",
             "providers": ["openai"],
             "owner_domains": ["contractor.com"] },
  "status": "active"
}'
```

Rule fields: `effect` (`deny` | `allow`), `providers[]` (name or type), `categories[]`,
`services[]` (slug), `owner_domains[]` (owner email domain).

### Quotas — seat / instance caps

Limit how many active grants a service may have. `enforcement: "block"` refuses
over-limit requests; `"warn"` allows them and records a warning.

```bash
curl -X POST localhost:8080/api/v1/ai/quotas -H 'Content-Type: application/json' -d '{
  "serviceSlug": "openai-api-access", "metric": "instances",
  "limitQuantity": 5, "period": "monthly", "enforcement": "block"
}'
```

(Seat/instance quotas are enforced at request time; token/cost quotas are enforced on
the [gateway](#proxy-real-usage--ai-gateway) path.)

### Budgets — spend gate

Set a USD limit for a scope (`global` | `provider` | `owner` | `workspace` | `tenant`).
At the **hard** threshold, new grants (and gateway calls) are blocked; at the **soft**
threshold, they are audited. Actuals are computed from usage records (incl. imported
provider costs).

```bash
curl -X POST localhost:8080/api/v1/ai/budgets -H 'Content-Type: application/json' -d '{
  "scope": "provider", "scopeRef": "openai-main", "limitUsd": 500,
  "period": "monthly", "softThresholdPct": 80, "hardThresholdPct": 100
}'
```

---

## Proxy real usage — AI Gateway

Let your team **use** an AI provider through OPORD without distributing the key. The
gateway forwards an OpenAI *Responses* call using the provider key, records usage and
audit metadata (not prompts/outputs), and honors the budget gate:

```bash
curl -X POST 'localhost:8080/api/v1/ai/gateway/openai/responses?provider=openai-main' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4.1-mini","input":"hello"}'
```

Import real provider spend for showback/budgets (OpenAI organization costs):

```bash
curl -X POST localhost:8080/api/v1/ai/usage/import/openai \
  -H 'Content-Type: application/json' -d '{"providerName":"openai-main"}'
```

---

## Configuration

Set on the `api` / `worker` services (env). All optional except `DATABASE_URL`
(the compose file sets it for you).

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_URL` | *(compose sets it)* | Postgres connection |
| `OPORD_HTTP_ADDR` | `:8080` | API listen address |
| `OPORD_AUTH_ENABLED` | `false` | API-key RBAC (off = single-team dev mode) |
| `OPENAI_API_KEY` | — | OpenAI key (Option A) |
| `ANTHROPIC_API_KEY` | — | Anthropic key (Option A) |
| `VAULT_ADDR`, `VAULT_TOKEN` | — | Secret store for `secret_ref` (Option B) |
| `OPORD_API_PORT` / `OPORD_WEB_PORT` | `8080` / `3000` | host ports to publish on |

Example — publish on different host ports:

```bash
OPORD_WEB_PORT=8088 OPORD_API_PORT=9090 \
  docker compose -f deployments/ai-compose.yml up -d
```

---

## Multi-user & authentication (optional)

By default **auth is off**: every caller is treated as an admin, which is fine for a
single-team self-host. To enable API-key RBAC (roles `viewer` < `operator` < `admin`;
reads need `viewer`+, writes need `operator`+), set `OPORD_AUTH_ENABLED=true` and seed
users with the CLI (`opord tenant add`, `opord user add`, which prints the key once).
The CLI is part of the source build (`go build ./cmd/cli`), not the container image.

---

## Manage the stack

```bash
docker compose -f deployments/ai-compose.yml ps            # status
docker compose -f deployments/ai-compose.yml logs -f api   # follow API logs
docker compose -f deployments/ai-compose.yml down          # stop (keep data)
docker compose -f deployments/ai-compose.yml down -v       # stop and wipe the database
```

Data (providers, requests, instances, policies, quotas, budgets, audit) persists in
the `opord_ai_pgdata` volume across restarts.

---

## What about infrastructure?

OPORD is also a multi-cloud **infrastructure** platform (provision VMs, Kubernetes
clusters, databases, networks, and full landing zones across AWS / Azure / GCP /
vSphere / Proxmox). In this **AI-first** build that surface is present in the codebase
but **not surfaced** in the UI — the console shows an "in development" placeholder when
the **AI** sign is off. The AI governance domain is intentionally a separate bounded
context and does **not** depend on the infrastructure side. Platform docs live in
`docs/` and `docs/adr/`.

---

## Architecture (short)

```
deployments/ai-compose.yml   db · migrate · api · worker · web (this stack)
cmd/{api,worker,cli}         entrypoints
internal/aiproviders         AI provider interface + OpenAI / Anthropic / MockAI
internal/orchestrator/ai*    AI lifecycle: requests, approval, instances,
                             enforcement (ai_enforce.go), budgets, gateway, audit
internal/api                 HTTP handlers (/api/v1/ai/*)
internal/{auth,creds,events,db}   RBAC, secret resolution, audit/connectors, sqlc
migrations/                  goose SQL (00021 = the AI governance domain + MockAI seed)
web/                         Next.js 16 console (the /ai/* workspace)
```

AI governance reuses the platform's request/approval workflow, RBAC, event bus, and
audit; AI data lives in dedicated `ai_*` tables. See
[ADR-0021](docs/adr/0021-ai-governance-domain.md).

---

## Build from source (development)

Requires Go ≥ 1.25, Node ≥ 20, Docker, and `goose`.

```bash
# infra only (Postgres + OpenBao), then build & run api/worker/web locally:
scripts/dev-up.sh      # idempotent: docker infra → migrate → build → api → worker → web
scripts/dev-down.sh    # stop
```

Conventions: conventional commits; Go wraps errors with `%w` and logs via `slog`; SQL
is raw via sqlc + goose (no ORM); TypeScript is strict (no `any`). Non-trivial
decisions get an ADR in `docs/adr/`.

---

## Security model

- **OPORD never stores raw provider keys.** Provider `config` rejects secret-looking
  keys; keys come from an environment variable or a secret store **by reference**
  (`secret_ref`), resolved per request.
- API responses **redact** any sensitive config keys.
- Governance decisions (allow / warn / **block**) are **audited** with actor and reason.
- Run behind your own ingress/TLS and enable `OPORD_AUTH_ENABLED` for multi-user use.
