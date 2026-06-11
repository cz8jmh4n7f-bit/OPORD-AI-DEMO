-- +goose Up
-- Reserve four new resource kinds backed by their first-class tofu modules:
--   s3      -> modules/aws-s3-bucket    (object storage)
--   secret  -> modules/aws-secret       (Secrets Manager + KMS)
--   queue   -> modules/aws-sqs          (SQS standard / FIFO)
--   cache   -> modules/aws-elasticache  (Redis replication group)
-- Models + tofu modules land first (scaffolding). The S3Provisioner /
-- SecretProvisioner / QueueProvisioner / CacheProvisioner capabilities + AWS
-- implementations + orchestrator wiring follow per primitive.
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table', 'function', 'project', 'account', 's3', 'secret', 'queue', 'cache'));

-- +goose Down
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table', 'function', 'project', 'account'));
