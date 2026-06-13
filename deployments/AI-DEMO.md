# OPORD - AI Service Governance (containerized)

A self-contained stack - PostgreSQL + the OPORD API, worker, and web console -
focused on **AI Service Governance**: govern who can access which AI services
(OpenAI, Anthropic, ...) with approval, expiry, **seat quotas, budgets, deny
policies**, usage metering, and a full audit trail.

## Run it

```bash
docker compose -f deployments/ai-compose.yml up --build
```

Open <http://localhost:3000>, click the **AI** sign, then **Enter the AI
workspace**. The catalog is seeded with **MockAI**, so it works with **no
secrets**.

To govern real providers, pass API keys (read via the env fallback):

```bash
OPENAI_API_KEY=sk-... ANTHROPIC_API_KEY=sk-ant-... \
  docker compose -f deployments/ai-compose.yml up --build
```

- Web: <http://localhost:3000>
- API: <http://localhost:8080> (auth disabled in this demo profile)
- Override host ports with `OPORD_WEB_PORT` / `OPORD_API_PORT`

## What's enforced

Every AI request is checked before access is granted:

- **Policies** - deny-list guardrails (by provider / category / service / owner domain)
- **Quotas** - seat/instance caps (`enforcement: block` refuses, `warn` audits)
- **Budgets** - spend gate (hard limit refuses; the gateway proxy path too)

Blocked requests return **403** with a reason and are recorded in the audit log.

## Stop

```bash
docker compose -f deployments/ai-compose.yml down      # keep data
docker compose -f deployments/ai-compose.yml down -v   # wipe data
```
