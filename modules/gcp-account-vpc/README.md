# `gcp-account-vpc` - project factory Layer: secure VPC

Layer 4 of the GCP project factory (ADR-0011). A locked-down VPC replacing the
(absent) default network.

- **VPC** `auto_create_subnetworks = false`, regional routing.
- **3 /24 subnets** carved from a **/22** (`vpc_cidr`, allocated by OPORD's IPAM
  from the OpenBao CIDR pool - `internal/ipam`, atomic CAS) via `cidrsubnet`.
  Private Google Access on; **VPC flow logs** (5s aggregation, 0.5 sampling).
- **ZTNA firewall** - 8 rules, explicit allow then deny, both directions:
  | prio | rule |
  |---|---|
  | 100 | allow SSH (22) from trusted |
  | 200 | allow RDP (3389) from trusted |
  | 300 | allow HTTPS (443) from trusted |
  | 400 | allow ICMP from trusted |
  | 500 | **deny all ingress** |
  | 600 | allow egress to org ranges |
  | 650 | allow egress to Google APIs (199.36.153.4/30, .8/30) |
  | 700 | **deny all egress** |

`trusted` = `allow_inbound_cidrs` (org IP ranges); dev default `0.0.0.0/0` (set
real ranges for prod).

## Inputs

`project_id`, `csa_id`, `region`, `vpc_cidr` (/22), `allow_inbound_cidrs`,
`subnet_count` (default 3).

## Why IPAM, not Vault-read-in-tofu

The /22 is allocated **before** this layer runs, by the orchestrator's
`internal/ipam` (OpenBao KV v2 compare-and-swap - atomic across concurrent
account creates), then passed in as `vpc_cidr`. The CIDR is released back to the
pool on account decommission. tofu never races the pool.

## Outputs

`network`, `network_name`, `subnets` (name to cidr), `vpc_cidr`.
