output "control_plane_ips" {
  description = "Static IPs assigned to control-plane nodes."
  value       = [for n in local.control_plane_nodes : n.ip]
}

output "worker_ips" {
  description = "Static IPs assigned to worker nodes."
  value       = [for n in local.worker_nodes : n.ip]
}

output "all_node_ips" {
  description = "All node IPs (control plane first, then workers)."
  value       = concat([for n in local.control_plane_nodes : n.ip], [for n in local.worker_nodes : n.ip])
}

output "control_plane_names" {
  description = "Hostnames of control-plane nodes."
  value       = [for n in local.control_plane_nodes : n.name]
}

output "worker_names" {
  description = "Hostnames of worker nodes."
  value       = [for n in local.worker_nodes : n.name]
}

output "control_plane_endpoint" {
  description = "Kubernetes API endpoint in host:port form."
  value       = "${var.control_plane_endpoint}:${var.control_plane_endpoint_port}"
}

output "ansible_inventory" {
  description = "Rendered Ansible inventory (INI) for the bootstrap phase."
  value       = local.ansible_inventory
}
