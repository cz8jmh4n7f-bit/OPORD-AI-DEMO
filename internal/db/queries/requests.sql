-- name: CreateRequest :one
insert into requests (name, environment, requester, kind, provider, blueprint, spec, tenant_id)
values ($1, $2, $3, $4, $5, $6, $7, $8)
returning *;

-- name: GetRequest :one
select * from requests where id = $1;

-- name: GetRequestByName :one
select * from requests where name = $1 and environment = $2;

-- name: ListRequests :many
select * from requests order by created_at desc;

-- name: UpdateRequestStatus :one
update requests
set status = $2, updated_at = now()
where id = $1
returning *;

-- name: SetRequestTicket :one
update requests
set ticket_ref = $2, updated_at = now()
where id = $1
returning *;

-- name: SetRequestResource :one
update requests
set resource_ref = $2, status = $3, updated_at = now()
where id = $1
returning *;

-- name: DecideRequest :one
update requests
set status = $2, decided_by = $3, reason = $4, updated_at = now()
where id = $1
returning *;
