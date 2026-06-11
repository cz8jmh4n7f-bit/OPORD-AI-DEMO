-- +goose Up
-- Add the generic "stack" resource kind: an arbitrary OpenTofu root module run
-- by OPORD (provision anything the provider supports - any AWS resource, etc.).
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack'));

-- +goose Down
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database'));
