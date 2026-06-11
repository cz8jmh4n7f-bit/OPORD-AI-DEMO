-- +goose Up
-- Provider connectivity health: the result of the most recent reachability +
-- credential check (POST /api/v1/providers/{name}/check). Persisting it lets
-- operators monitor backend health over time (and lets the UI show a status
-- badge) without re-probing the backend on every read.
alter table providers
    add column last_check_status     text    not null default '',
    add column last_check_message    text    not null default '',
    add column last_check_latency_ms integer not null default 0,
    add column last_check_at         timestamptz;

-- +goose Down
alter table providers
    drop column last_check_status,
    drop column last_check_message,
    drop column last_check_latency_ms,
    drop column last_check_at;
