# modules/proxmox-vm

**Original OPORD OpenTofu module** for provisioning standalone VMs on **Proxmox
VE**, via the `bpg/proxmox` provider - the generic "vm" blueprint for Proxmox
(parity with `modules/vsphere-vm`).

Clones a template (`template_vmid`) into `vm_count` VMs with cores/memory/disk
and a cloud-init static IP. Emits names, VMIDs, and IPs.

## State

`pg` backend, one workspace per resource:

```bash
tofu init -backend-config="conn_str=postgres://opord:opord@localhost:5432/opord?sslmode=disable"
tofu workspace new <resource-id>
```

## Key inputs

`proxmox_endpoint` (e.g. `https://pve:8006/`), `proxmox_username`/`password`,
`node_name`, `template_vmid`, `vm_count`, `cores`, `memory_mb`, `disk_gb`,
`datastore_id` (default `local-lvm`), `network_bridge` (default `vmbr0`),
`ip_start` + `netmask_bits` + `gateway`, `dns_servers`.

## Outputs

`vm_names`, `vm_ids`, `vm_ips`.

## Notes

- Clone source is a **template VMID** (integer), not a name.
- IP is configured via cloud-init (`initialization.ip_config`); the template
  must be a cloud-init-enabled image.
- `proxmox_insecure = true` accepts self-signed Proxmox TLS.
