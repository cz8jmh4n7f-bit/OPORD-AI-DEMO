-- +goose Up
-- Tenant data-scoping for the generic resources table (vm/database/stack).
-- Nullable: resources created without an authenticated tenant (CLI/dev) stay
-- global. Non-admin API callers only see their tenant's resources.
alter table resources add column tenant_id uuid references tenants (id) on delete set null;
create index idx_resources_tenant on resources (tenant_id);

-- +goose Down
alter table resources drop column if exists tenant_id;
