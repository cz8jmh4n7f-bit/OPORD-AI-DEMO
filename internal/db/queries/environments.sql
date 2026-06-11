-- name: CreateEnvironment :one
insert into environments (name, environment, blueprint, spec, tenant_id)
values ($1, $2, $3, $4, $5)
returning *;

-- name: GetEnvironment :one
select * from environments where id = $1;

-- name: GetEnvironmentByName :one
select * from environments where name = $1 and environment = $2;

-- name: ListEnvironments :many
select * from environments order by created_at desc;

-- name: UpdateEnvironmentStatus :one
update environments
set status = $2, updated_at = now()
where id = $1
returning *;

-- name: DeleteEnvironment :exec
delete from environments where id = $1;
