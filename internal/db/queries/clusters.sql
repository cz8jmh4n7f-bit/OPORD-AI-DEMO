-- name: CreateCluster :one
insert into clusters (name, environment, provider_id, desired_spec, tofu_workspace, tenant_id)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: UpdateClusterSpec :one
update clusters
set desired_spec = $2, updated_at = now()
where id = $1
returning *;

-- name: GetCluster :one
select * from clusters where id = $1;

-- name: GetClusterByName :one
select * from clusters where name = $1 and environment = $2;

-- name: ListClusters :many
select * from clusters order by created_at desc;

-- name: ListClustersByProvider :many
select * from clusters where provider_id = $1 order by created_at desc;

-- name: UpdateClusterStatus :one
update clusters
set status = $2, updated_at = now()
where id = $1
returning *;

-- name: UpdateClusterState :one
update clusters
set observed_state = $2, kubeconfig_ref = $3, updated_at = now()
where id = $1
returning *;

-- name: DeleteCluster :exec
delete from clusters where id = $1;
