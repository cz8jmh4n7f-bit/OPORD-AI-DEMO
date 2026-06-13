output "control_plane_ips" {
  value = [for n in local.control_plane_nodes : n.ip]
}

output "worker_ips" {
  value = [for n in local.worker_nodes : n.ip]
}

output "all_node_ips" {
  value = concat([for n in local.control_plane_nodes : n.ip], [for n in local.worker_nodes : n.ip])
}

output "control_plane_names" {
  value = [for n in local.control_plane_nodes : n.name]
}

output "worker_names" {
  value = [for n in local.worker_nodes : n.name]
}

output "control_plane_endpoint" {
  value = "${var.control_plane_endpoint}:${var.control_plane_endpoint_port}"
}

output "ansible_inventory" {
  value = local.ansible_inventory
}
