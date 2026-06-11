-- +goose Up
-- Composed environments (EaaS Layer 2): a named bundle of resources/clusters
-- expanded from a blueprint and managed as one unit. Membership is logical -
-- each component is a normal cluster/resource named "<env>-<component>" - so the
-- clusters/resources tables are unchanged; the expanded spec lives in spec.

create table environments (
    id          uuid primary key default gen_random_uuid(),
    name        text not null,
    environment text not null default 'dev',
    blueprint   text not null,
    status      text not null default 'pending'
                check (status in ('pending', 'provisioning', 'ready',
                                  'degraded', 'destroying', 'destroyed', 'failed')),
    spec        jsonb not null default '{}'::jsonb,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    unique (name, environment)
);

create index idx_environments_blueprint on environments (blueprint);

-- +goose Down
drop table if exists environments;
