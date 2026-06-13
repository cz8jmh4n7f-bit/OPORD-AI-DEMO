# ansible/

**OPORD Phase 2 - Kubernetes bootstrap.** Provider-agnostic: runs over whatever
VMs a provider produced, using the inventory emitted in
`ProvisionResult.AnsibleInventory` (groups: `control_plane_init`,
`control_plane_join`, `control_plane`, `workers`, `k8s_cluster`).

These roles/playbooks are adapted from the owner's `k8s-platform` project, with
two changes for OPORD: the kubeconfig owner is `admin_user` (derived from the
inventory `ansible_user`) instead of a hardcoded account, and the API port
variable is standardized to `control_plane_endpoint_port` everywhere.

## Run order (or just run `site.yml`)

| Playbook | Hosts | Does |
|----------|-------|------|
| `base.yml` | `k8s_cluster` | common OS prep, containerd, kube{adm,let,ctl} packages |
| `bootstrap.yml` | `control_plane_init` | `kubeadm init` on the first control plane |
| `join-control-planes.yml` | `control_plane_join` | join additional CPs (serial, HA) |
| `join-workers.yml` | `workers` | join workers |
| `install-cilium.yml` | `control_plane_init` | Cilium CNI + L2 announcements + LB pool |
| `site.yml` | - | imports all of the above in order |

## Roles

`common`, `containerd`, `kubernetes_prereqs`, `kubeadm_init`. Versions and CIDRs
live in `group_vars/all.yml`.

## Assumptions

- The golden VM template carries the SSH key, helm, and kubectl (Packer pattern).
- `cilium_l2_interface` and the LB pool in `install-cilium.yml` are environment
  defaults - override per cluster as needed.

## Not yet ported (day-2, available in k8s-platform)

ArgoCD, MetalLB/kube-vip, metrics-server, external-secrets, Longhorn, agents,
etcd backup/restore, cert rotation, upgrades.
