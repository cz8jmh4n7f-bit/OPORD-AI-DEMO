-- name: CreateTenant :one
insert into tenants (name) values ($1) returning *;

-- name: GetTenantByName :one
select * from tenants where name = $1;

-- name: GetTenant :one
select * from tenants where id = $1;

-- name: ListTenants :many
select * from tenants order by created_at desc;

-- name: CreateUser :one
insert into users (email, tenant_id, role, api_key_hash)
values ($1, $2, $3, $4)
returning *;

-- name: GetUserByEmail :one
select * from users where email = $1;

-- name: GetUserByAPIKeyHash :one
select * from users where api_key_hash = $1;

-- name: ListUsers :many
select * from users order by created_at desc;

-- name: CountUsers :one
select count(*) from users;
