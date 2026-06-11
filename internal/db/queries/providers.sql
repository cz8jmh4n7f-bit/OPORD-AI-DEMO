-- name: CreateProvider :one
insert into providers (name, type, config, secret_ref)
values ($1, $2, $3, $4)
returning *;

-- name: GetProvider :one
select * from providers where id = $1;

-- name: GetProviderByName :one
select * from providers where name = $1;

-- name: ListProviders :many
select * from providers order by created_at desc;

-- name: CountClustersByProvider :one
select count(*) from clusters where provider_id = $1;

-- name: CountResourcesByProvider :one
select count(*) from resources where provider_id = $1;

-- name: UpdateProvider :one
update providers
set name = $2, type = $3, config = $4, secret_ref = $5, updated_at = now()
where id = $1
returning *;

-- name: UpdateProviderHealth :one
update providers
set last_check_status = $2,
    last_check_message = $3,
    last_check_latency_ms = $4,
    last_check_at = now()
where id = $1
returning *;

-- name: DeleteProvider :exec
delete from providers where id = $1;
