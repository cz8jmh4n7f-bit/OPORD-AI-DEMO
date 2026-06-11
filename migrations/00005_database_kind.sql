-- +goose Up
-- Add the managed-database resource kind (e.g. AWS RDS). Databases live in the
-- generic resources table like VMs, with kind='database'.
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database'));

-- +goose Down
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster'));
