-- +goose Up
-- Multi-tenancy + RBAC foundation: tenants (org/team boundary) and users with a
-- role and an API key (stored hashed). Per-resource tenant scoping is layered on
-- later; this establishes identity + authorization.

create table tenants (
    id         uuid primary key default gen_random_uuid(),
    name       text not null unique,
    created_at timestamptz not null default now()
);

create table users (
    id           uuid primary key default gen_random_uuid(),
    email        text not null unique,
    tenant_id    uuid not null references tenants (id) on delete cascade,
    role         text not null default 'viewer' check (role in ('admin', 'operator', 'viewer')),
    api_key_hash text not null default '',
    created_at   timestamptz not null default now()
);

create index idx_users_apikey on users (api_key_hash);
create index idx_users_tenant on users (tenant_id);

-- +goose Down
drop table if exists users;
drop table if exists tenants;
