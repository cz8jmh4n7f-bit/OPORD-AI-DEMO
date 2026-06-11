-- +goose Up
-- ADR-0016: the AWS expose-layer. Five new resource kinds, each backed by a
-- first-class tofu module + provider capability:
--   dns          -> modules/aws-route53-zone (Route53 hosted zone + records)
--   cert         -> modules/aws-acm-cert      (ACM certificate, DNS-validated)
--   loadbalancer -> modules/aws-alb           (Application Load Balancer)
--   apigateway   -> modules/aws-apigw         (API Gateway v2 HTTP API)
--   cdn          -> modules/aws-cloudfront     (CloudFront distribution)
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table', 'function', 'project', 'account', 's3', 'secret', 'queue', 'cache', 'dns', 'cert', 'loadbalancer', 'apigateway', 'cdn'));

-- +goose Down
alter table resources drop constraint if exists resources_kind_check;
alter table resources add constraint resources_kind_check check (kind in ('vm', 'k8s-cluster', 'database', 'stack', 'table', 'function', 'project', 'account', 's3', 'secret', 'queue', 'cache'));
