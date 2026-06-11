-- name: CreateBackup :one
insert into backups (resource_kind, resource_name, environment, provider, tofu_workspace, tenant_id)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: GetBackup :one
select * from backups where id = $1;

-- name: ListBackups :many
select * from backups order by created_at desc;

-- name: SetBackupResult :one
update backups
set snapshot_id = $2, status = $3, updated_at = now()
where id = $1
returning *;
