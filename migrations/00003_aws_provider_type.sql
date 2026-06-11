-- +goose Up
alter table providers drop constraint if exists providers_type_check;
alter table providers add constraint providers_type_check check (type in ('vsphere', 'proxmox', 'aws'));

-- +goose Down
alter table providers drop constraint if exists providers_type_check;
alter table providers add constraint providers_type_check check (type in ('vsphere', 'proxmox'));
