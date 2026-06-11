-- name: UpsertNode :one
insert into nodes (cluster_id, name, role, ip_address, vm_moid, cpu, memory_mb, disk_gb, status)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
on conflict (cluster_id, name) do update
set role = excluded.role,
    ip_address = excluded.ip_address,
    vm_moid = excluded.vm_moid,
    cpu = excluded.cpu,
    memory_mb = excluded.memory_mb,
    disk_gb = excluded.disk_gb,
    status = excluded.status,
    updated_at = now()
returning *;

-- name: ListNodesByCluster :many
select * from nodes where cluster_id = $1 order by role, name;

-- name: UpdateNodeStatus :one
update nodes
set status = $2, updated_at = now()
where id = $1
returning *;

-- name: DeleteNodesByCluster :exec
delete from nodes where cluster_id = $1;
