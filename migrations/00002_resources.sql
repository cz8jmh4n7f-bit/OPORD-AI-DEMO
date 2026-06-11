-- +goose Up
-- Generic resource model: any provisionable thing (vm, k8s-cluster, ...) is a
-- resource with a kind + declarative spec. The k8s `clusters` table stays for
-- now; new kinds (starting with vm) live here.

create table resources (
    id             uuid primary key default gen_random_uuid(),
    name           text not null,
    environment    text not null default 'dev',
    provider_id    uuid not null references providers (id) on delete restrict,
    kind           text not null check (kind in ('vm', 'k8s-cluster')),
    status         text not null default 'pending'
                   check (status in ('pending', 'provisioning', 'ready',
                                     'degraded', 'destroying', 'destroyed', 'failed')),
    spec           jsonb not null default '{}'::jsonb,
    observed       jsonb not null default '{}'::jsonb,
    tofu_workspace text not null unique,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now(),
    unique (name, environment)
);

create index idx_resources_provider on resources (provider_id);
create index idx_resources_kind on resources (kind);

-- +goose Down
drop table if exists resources;
