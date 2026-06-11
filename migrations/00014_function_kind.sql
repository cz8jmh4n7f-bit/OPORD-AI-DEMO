-- +goose Up
-- Add the "function" resource kind: a serverless function (AWS Lambda today).
-- First-class catalog primitive alongside vm / database / stack / table.
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table', 'function'));

-- +goose Down
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table'));
