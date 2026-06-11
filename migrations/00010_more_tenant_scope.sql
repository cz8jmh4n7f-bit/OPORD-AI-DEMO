-- +goose Up
-- Extend tenant data-scoping to clusters, environments and requests (resources
-- were scoped in 00009). Nullable: rows created without an authenticated tenant
-- (CLI/dev) stay global.
alter table clusters add column tenant_id uuid references tenants (id) on delete set null;
alter table environments add column tenant_id uuid references tenants (id) on delete set null;
alter table requests add column tenant_id uuid references tenants (id) on delete set null;

create index idx_clusters_tenant on clusters (tenant_id);
create index idx_environments_tenant on environments (tenant_id);
create index idx_requests_tenant on requests (tenant_id);

-- +goose Down
alter table clusters drop column if exists tenant_id;
alter table environments drop column if exists tenant_id;
alter table requests drop column if exists tenant_id;
