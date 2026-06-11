-- +goose Up
-- Add the "project" resource kind: an access-vending project (IAM Identity
-- Center group + permission set + account assignment). First-class catalog
-- primitive alongside vm / database / stack / table / function.
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table', 'function', 'project'));

-- +goose Down
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table', 'function'));
