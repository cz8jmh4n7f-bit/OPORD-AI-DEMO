-- +goose Up
-- Core OPORD domain schema. River manages its own tables via `river migrate`.
-- Status/role/type are text + CHECK rather than PG enums for migration flexibility.

create table providers (
    id         uuid primary key default gen_random_uuid(),
    name       text not null unique,
    type       text not null check (type in ('vsphere', 'proxmox')),
    -- non-secret connection config: server host, datacenter, cluster, datastore, network, folder
    config     jsonb not null default '{}'::jsonb,
    -- Vault path holding this provider's credentials (e.g. opord/vsphere/dev)
    secret_ref text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table templates (
    id            uuid primary key default gen_random_uuid(),
    name          text not null,
    os            text not null,
    os_version    text not null,
    provider_type text not null check (provider_type in ('vsphere', 'proxmox')),
    created_at    timestamptz not null default now(),
    unique (name, provider_type)
);

create table clusters (
    id             uuid primary key default gen_random_uuid(),
    name           text not null,
    environment    text not null default 'dev',
    provider_id    uuid not null references providers (id) on delete restrict,
    status         text not null default 'pending'
                   check (status in ('pending', 'provisioning', 'bootstrapping',
                                     'ready', 'degraded', 'destroying', 'destroyed', 'failed')),
    -- desired declarative spec: cp/worker counts, node specs, networking, k8s version, cni, template
    desired_spec   jsonb not null default '{}'::jsonb,
    -- last observed/applied state: tofu outputs, node inventory, etc.
    observed_state jsonb not null default '{}'::jsonb,
    -- isolated Tofu workspace name for this cluster's state
    tofu_workspace text not null unique,
    -- Vault path of the cluster kubeconfig once bootstrapped
    kubeconfig_ref text,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now(),
    unique (name, environment)
);

create table nodes (
    id         uuid primary key default gen_random_uuid(),
    cluster_id uuid not null references clusters (id) on delete cascade,
    name       text not null,
    role       text not null check (role in ('control_plane', 'worker')),
    ip_address text,
    vm_moid    text,
    cpu        int,
    memory_mb  int,
    disk_gb    int,
    status     text not null default 'pending'
               check (status in ('pending', 'provisioned', 'ready', 'failed')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (cluster_id, name)
);

create table jobs (
    id           uuid primary key default gen_random_uuid(),
    cluster_id   uuid references clusters (id) on delete cascade,
    operation    text not null check (operation in ('provision', 'bootstrap', 'reconcile', 'destroy')),
    status       text not null default 'queued'
                 check (status in ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    river_job_id bigint,
    error        text,
    started_at   timestamptz,
    finished_at  timestamptz,
    created_at   timestamptz not null default now()
);

create index idx_clusters_provider on clusters (provider_id);
create index idx_nodes_cluster on nodes (cluster_id);
create index idx_jobs_cluster on jobs (cluster_id);
create index idx_jobs_status on jobs (status);

-- +goose Down
drop table if exists jobs;
drop table if exists nodes;
drop table if exists clusters;
drop table if exists templates;
drop table if exists providers;
