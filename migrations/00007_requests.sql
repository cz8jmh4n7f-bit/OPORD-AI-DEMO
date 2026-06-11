-- +goose Up
-- Self-service requests: a user asks for a resource/environment; OPORD opens a
-- GLPI ticket; on approval it provisions (which then flows to CMDB + Slack via
-- the connector bus). The approval gate is the "head" of the request to ticket to 
-- job to CMDB to notify workflow.

create table requests (
    id           uuid primary key default gen_random_uuid(),
    name         text not null,
    environment  text not null default 'dev',
    requester    text not null default '',
    kind         text not null check (kind in ('vm', 'cluster', 'database', 'stack', 'environment')),
    provider     text not null,
    blueprint    text not null default '',
    spec         jsonb not null default '{}'::jsonb,
    status       text not null default 'pending_approval'
                 check (status in ('pending_approval', 'approved', 'rejected',
                                   'provisioning', 'completed', 'failed')),
    ticket_ref   text not null default '',
    resource_ref text not null default '',
    decided_by   text not null default '',
    reason       text not null default '',
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    unique (name, environment)
);

create index idx_requests_status on requests (status);

-- +goose Down
drop table if exists requests;
