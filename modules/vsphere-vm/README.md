# modules/vsphere-vm

**Original OPORD OpenTofu module** for provisioning **standalone VMs** on VMware
vSphere - the generic "vm" blueprint (no Kubernetes).

Clones a golden VM template (built by your Packer pipeline) into `vm_count`
plain VMs with static IPs, sizing, and optional data disks. Emits names, IPs,
managed-object IDs, and an Ansible inventory for optional post-provision config.

## State

`pg` backend, one workspace per resource (see `docs/adr/0003-state-isolation.md`):

```bash
tofu init -backend-config="conn_str=postgres://opord:opord@localhost:5432/opord?sslmode=disable"
tofu workspace new <resource-id>
```

## Key inputs

`template_name` (existing golden image), `vm_count`, `name_prefix`, `specs`
(cpu/memory/disk), `data_disks`, `ip_start` + `netmask_bits` + `gateway` +
`dns_servers`, placement (`vsphere_datacenter`/`cluster`/`datastore`/`network`/
`folder`).

## Outputs

`vm_names`, `vm_ips`, `vm_moids`, `ansible_inventory`.

## Notes

- Templates are produced separately (your Packer); this module only clones them.
- The SSH key is expected to be baked into the template; `ssh_public_key` is
  accepted for reference only.
- `firmware` must match the template (default `efi`).
