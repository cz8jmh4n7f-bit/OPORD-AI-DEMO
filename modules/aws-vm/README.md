# modules/aws-vm

**Original OPORD OpenTofu module** for standalone VMs (EC2 instances) on **AWS**,
via the `hashicorp/aws` provider - the generic "vm" blueprint for AWS.

Launches `vm_count` instances from an AMI with a chosen `instance_type`, optional
subnet / security groups / key pair, and a gp3 root volume.

## State

`pg` backend, one workspace per resource. AWS credentials are read from the
ambient environment (`AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` or a profile).

## Key inputs

`region`, `ami`, `instance_type` (default `t3.medium`), `vm_count`,
`name_prefix`, `root_volume_gb`, optional `subnet_id` / `security_group_ids` /
`key_name`, `associate_public_ip`.

## Outputs

`vm_names`, `vm_ids`, `private_ips`, `public_ips`.

## Note on the OPORD VMSpec mapping

Cloud sizing is by **instance type**, not raw vCPU/RAM. The AWS provider maps the
generic spec's `template` to AMI and reads `instance_type` from the provider
config (the form's vCPU/RAM are advisory for AWS).
