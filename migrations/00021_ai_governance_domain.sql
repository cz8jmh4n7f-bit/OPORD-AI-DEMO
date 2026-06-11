-- +goose Up
-- AI governance MVP: AI providers/services/instances live in their own bounded
-- domain, while requests reuse the existing approval workflow via kind=ai_service.

alter table requests drop constraint if exists requests_kind_check;
alter table requests add constraint requests_kind_check
    check (kind in ('vm', 'cluster', 'database', 'stack', 'environment', 'project', 'account', 'ai_service'));

create table ai_providers (
    id         uuid primary key default gen_random_uuid(),
    name       text not null unique,
    type       text not null check (type in ('mock_ai', 'openai', 'anthropic', 'gemini', 'github_copilot', 'cursor')),
    config     jsonb not null default '{}'::jsonb,
    status     text not null default 'active' check (status in ('active', 'disabled')),
    tenant_id  uuid references tenants (id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table ai_provider_credentials (
    id                     uuid primary key default gen_random_uuid(),
    provider_id            uuid not null references ai_providers (id) on delete cascade,
    secret_ref             text not null default '',
    scopes                 text[] not null default '{}'::text[],
    rotation_due_at        timestamptz,
    last_validated_at      timestamptz,
    last_validation_status text not null default '',
    created_at             timestamptz not null default now()
);

create table ai_services (
    id                      uuid primary key default gen_random_uuid(),
    provider_id             uuid not null references ai_providers (id) on delete cascade,
    name                    text not null,
    slug                    text not null,
    category                text not null default 'access',
    description             text not null default '',
    request_schema          jsonb not null default '{}'::jsonb,
    default_expiration_days integer not null default 30,
    requires_approval       boolean not null default true,
    status                  text not null default 'active' check (status in ('active', 'disabled')),
    created_at              timestamptz not null default now(),
    updated_at              timestamptz not null default now(),
    unique (provider_id, slug)
);

create table ai_service_instances (
    id                 uuid primary key default gen_random_uuid(),
    service_id         uuid not null references ai_services (id) on delete restrict,
    request_id         uuid references requests (id) on delete set null,
    provider_access_id text not null default '',
    owner              text not null default '',
    tenant_id          uuid references tenants (id) on delete set null,
    workspace          text not null default '',
    status             text not null default 'provisioning'
                       check (status in ('provisioning', 'active', 'suspended', 'revoking', 'revoked', 'expired', 'failed')),
    spec               jsonb not null default '{}'::jsonb,
    observed           jsonb not null default '{}'::jsonb,
    provisioned_at     timestamptz,
    expires_at         timestamptz,
    revoked_at         timestamptz,
    created_at         timestamptz not null default now(),
    updated_at         timestamptz not null default now()
);

create table ai_usage_records (
    id           uuid primary key default gen_random_uuid(),
    instance_id  uuid references ai_service_instances (id) on delete cascade,
    provider_id  uuid not null references ai_providers (id) on delete cascade,
    period_start timestamptz not null,
    period_end   timestamptz not null,
    metric       text not null,
    quantity     double precision not null default 0,
    unit         text not null default '',
    cost_usd     double precision not null default 0,
    raw          jsonb not null default '{}'::jsonb,
    created_at   timestamptz not null default now()
);

create table ai_budgets (
    id                 uuid primary key default gen_random_uuid(),
    tenant_id          uuid references tenants (id) on delete cascade,
    scope              text not null,
    scope_ref          text not null,
    limit_usd          double precision not null,
    period             text not null default 'monthly',
    soft_threshold_pct integer not null default 80,
    hard_threshold_pct integer not null default 100,
    created_at         timestamptz not null default now(),
    updated_at         timestamptz not null default now()
);

create table ai_quotas (
    id             uuid primary key default gen_random_uuid(),
    service_id     uuid references ai_services (id) on delete cascade,
    tenant_id      uuid references tenants (id) on delete cascade,
    metric         text not null,
    limit_quantity double precision not null,
    period         text not null default 'monthly',
    enforcement    text not null default 'warn' check (enforcement in ('warn', 'block')),
    created_at     timestamptz not null default now()
);

create table ai_access_policies (
    id         uuid primary key default gen_random_uuid(),
    name       text not null unique,
    tenant_id  uuid references tenants (id) on delete cascade,
    rules      jsonb not null default '{}'::jsonb,
    status     text not null default 'active' check (status in ('active', 'disabled')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table ai_model_catalog (
    id          uuid primary key default gen_random_uuid(),
    provider_id uuid not null references ai_providers (id) on delete cascade,
    model       text not null,
    display_name text not null default '',
    modality    text not null default 'text',
    status      text not null default 'active' check (status in ('active', 'disabled')),
    metadata    jsonb not null default '{}'::jsonb,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    unique (provider_id, model)
);

create table ai_audit_events (
    id           uuid primary key default gen_random_uuid(),
    actor        text not null default '',
    tenant_id    uuid references tenants (id) on delete set null,
    subject_type text not null,
    subject_id   uuid,
    action       text not null,
    message      text not null default '',
    fields       jsonb not null default '{}'::jsonb,
    created_at   timestamptz not null default now()
);

create index idx_ai_providers_tenant on ai_providers (tenant_id);
create index idx_ai_services_provider on ai_services (provider_id);
create index idx_ai_instances_service on ai_service_instances (service_id);
create index idx_ai_instances_status on ai_service_instances (status);
create index idx_ai_instances_tenant on ai_service_instances (tenant_id);
create index idx_ai_usage_instance on ai_usage_records (instance_id);
create index idx_ai_usage_provider_period on ai_usage_records (provider_id, period_start, period_end);
create index idx_ai_audit_subject on ai_audit_events (subject_type, subject_id);
create index idx_ai_audit_tenant_created on ai_audit_events (tenant_id, created_at desc);

insert into ai_providers (name, type, config)
values ('mock-ai', 'mock_ai', '{"mvp": true}'::jsonb);

insert into ai_services (provider_id, name, slug, category, description, request_schema, default_expiration_days, requires_approval)
select id, 'OpenAI API Access (Mock)', 'openai-api-mock', 'api_access',
       'Mock governed access to OpenAI-style API services for MVP testing.',
       '{"fields":["owner","workspace","justification","expires_at"]}'::jsonb, 30, true
from ai_providers where name = 'mock-ai';

insert into ai_services (provider_id, name, slug, category, description, request_schema, default_expiration_days, requires_approval)
select id, 'Claude Access (Mock)', 'claude-access-mock', 'api_access',
       'Mock governed access to Claude-style services for MVP testing.',
       '{"fields":["owner","workspace","justification","expires_at"]}'::jsonb, 30, true
from ai_providers where name = 'mock-ai';

insert into ai_services (provider_id, name, slug, category, description, request_schema, default_expiration_days, requires_approval)
select id, 'Kubernetes AI Sandbox (Mock)', 'k8s-ai-sandbox-mock', 'sandbox',
       'Mock AI sandbox entitlement; no cluster or external provider is created.',
       '{"fields":["owner","workspace","justification","expires_at"]}'::jsonb, 7, true
from ai_providers where name = 'mock-ai';

insert into ai_model_catalog (provider_id, model, display_name, modality, metadata)
select id, 'mock-gpt-4.1-mini', 'Mock GPT-4.1 mini', 'text', '{"mock": true}'::jsonb
from ai_providers where name = 'mock-ai';

insert into ai_model_catalog (provider_id, model, display_name, modality, metadata)
select id, 'mock-embedding-small', 'Mock Embedding Small', 'embedding', '{"mock": true}'::jsonb
from ai_providers where name = 'mock-ai';

-- +goose Down
delete from ai_model_catalog where provider_id in (select id from ai_providers where name = 'mock-ai');
delete from ai_services where provider_id in (select id from ai_providers where name = 'mock-ai');
delete from ai_providers where name = 'mock-ai';

drop table if exists ai_audit_events;
drop table if exists ai_model_catalog;
drop table if exists ai_access_policies;
drop table if exists ai_quotas;
drop table if exists ai_budgets;
drop table if exists ai_usage_records;
drop table if exists ai_service_instances;
drop table if exists ai_services;
drop table if exists ai_provider_credentials;
drop table if exists ai_providers;

alter table requests drop constraint if exists requests_kind_check;
alter table requests add constraint requests_kind_check
    check (kind in ('vm', 'cluster', 'database', 'stack', 'environment'));
