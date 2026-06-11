-- name: CreateResource :one
insert into resources (name, environment, provider_id, kind, spec, tofu_workspace, tenant_id)
values ($1, $2, $3, $4, $5, $6, $7)
returning *;

-- name: GetResource :one
select * from resources where id = $1;

-- name: GetResourceByName :one
select * from resources where name = $1 and environment = $2;

-- name: ListResources :many
select * from resources order by created_at desc;

-- name: ListResourcesByKind :many
select * from resources where kind = $1 order by created_at desc;

-- name: UpdateResourceStatus :one
update resources
set status = $2, updated_at = now()
where id = $1
returning *;

-- name: UpdateResourceSpec :one
update resources
set spec = $2, status = $3, updated_at = now()
where id = $1
returning *;

-- name: UpdateResourceObserved :one
update resources
set observed = $2, status = $3, updated_at = now()
where id = $1
returning *;

-- name: DeleteResource :exec
delete from resources where id = $1;
