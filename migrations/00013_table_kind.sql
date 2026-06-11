-- +goose Up
-- Add the "table" resource kind: a managed NoSQL table (AWS DynamoDB today).
-- First-class catalog primitive alongside vm / database / stack.
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table'));

-- +goose Down
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack'));
