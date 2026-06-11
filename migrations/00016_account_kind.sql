-- +goose Up
-- Add the "account" resource kind: a provisioned member AWS account (Organizations
-- CreateAccount + multi-layer baseline). The orchestrator drives layers L1-L6;
-- per-layer status is stored in resources.observed.
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table', 'function', 'project', 'account'));

-- +goose Down
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table', 'function', 'project'));
