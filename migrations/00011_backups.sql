-- +goose Up
-- Day-2 backups: a snapshot is an action recorded here. Each backup runs in its
-- own Tofu workspace (so it can be listed/destroyed independently of the source).
create table backups (
    id             uuid primary key default gen_random_uuid(),
    resource_kind  text not null,
    resource_name  text not null,
    environment    text not null default 'dev',
    provider       text not null default '',
    snapshot_id    text not null default '',
    tofu_workspace text not null unique,
    status         text not null default 'pending'
                   check (status in ('pending', 'running', 'completed', 'failed')),
    tenant_id      uuid references tenants (id) on delete set null,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now()
);

create index idx_backups_resource on backups (resource_kind, resource_name);

-- +goose Down
drop table if exists backups;
