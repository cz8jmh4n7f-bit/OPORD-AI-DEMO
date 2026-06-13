-- +goose Up
-- Agent & MCP governance: organizations are deploying AI agents that connect to
-- MCP (Model Context Protocol) servers / tool endpoints, with no control over
-- WHICH servers a team may use. This is the governance layer: a registry of
-- approved MCP servers (with a risk tier + an allowed-tool list) and per-team
-- grants, enforced by an authorize endpoint an agent runtime calls before it
-- connects. Audit reuses ai_audit_events (subject_type = 'mcp_server'/'mcp_grant').

create table mcp_servers (
    id            uuid primary key default gen_random_uuid(),
    name          text not null unique,
    transport     text not null default 'stdio' check (transport in ('stdio', 'http', 'sse')),
    endpoint      text not null default '',
    description   text not null default '',
    risk_tier     text not null default 'medium' check (risk_tier in ('low', 'medium', 'high', 'critical')),
    allowed_tools jsonb not null default '[]'::jsonb,
    status        text not null default 'active' check (status in ('active', 'disabled')),
    tenant_id     uuid references tenants (id) on delete set null,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);

create table mcp_grants (
    id         uuid primary key default gen_random_uuid(),
    server_id  uuid not null references mcp_servers (id) on delete cascade,
    owner      text not null,
    status     text not null default 'active' check (status in ('active', 'revoked', 'expired')),
    expires_at timestamptz,
    granted_by text not null default '',
    tenant_id  uuid references tenants (id) on delete set null,
    created_at timestamptz not null default now(),
    revoked_at timestamptz
);

create index mcp_grants_server_owner_idx on mcp_grants (server_id, owner);

-- +goose Down
drop table if exists mcp_grants;
drop table if exists mcp_servers;
