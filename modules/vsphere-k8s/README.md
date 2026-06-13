# modules/vsphere-k8s

**Original OPORD OpenTofu module**, written from the public `hashicorp/vsphere`
provider documentation. It is OPORD's own code - not copied from any other
repository.

## What it does (Phase 1 - infrastructure only)

Clones a golden VM template into N control-plane + M worker VMs on VMware
vSphere, assigns static IPs (incrementing the last octet from `*_ip_start`),
and emits the data the Ansible bootstrap phase needs.

It does **not** install Kubernetes - that is Phase 2 (Ansible), kept separate so
the configuration step is provider-agnostic (see
`docs/adr/0002-provider-abstraction.md`).

## State

Uses the `pg` backend with one workspace per cluster
(`docs/adr/0003-state-isolation.md`). Initialize with:

```bash
tofu init -backend-config="conn_str=postgres://opord:opord@localhost:5432/opord?sslmode=disable"
tofu workspace new <cluster-id>
```

## Key outputs

`control_plane_ips`, `worker_ips`, `all_node_ips`, `control_plane_names`,
`worker_names`, `control_plane_endpoint` (`host:port`), and `ansible_inventory`
(rendered INI consumed by Phase 2).

## Notes

- The SSH authorized key is expected to be baked into the golden template (the
  standard Packer pattern); `ssh_public_key` is accepted for reference only.
- `firmware` must match the template (default `efi`).
