-- name: CreateJob :one
insert into jobs (cluster_id, operation, status)
values ($1, $2, 'queued')
returning *;

-- name: GetJob :one
select * from jobs where id = $1;

-- name: ListJobsByCluster :many
select * from jobs where cluster_id = $1 order by created_at desc;

-- name: MarkJobRunning :one
update jobs
set status = 'running', river_job_id = $2, started_at = now()
where id = $1
returning *;

-- name: MarkJobFinished :one
update jobs
set status = $2, error = $3, finished_at = now()
where id = $1
returning *;
