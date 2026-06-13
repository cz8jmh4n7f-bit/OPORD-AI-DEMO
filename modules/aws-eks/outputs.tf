output "cluster_name" {
  description = "EKS cluster name."
  value       = aws_eks_cluster.this.name
}

output "cluster_endpoint" {
  description = "EKS API server endpoint."
  value       = aws_eks_cluster.this.endpoint
}

output "cluster_ca" {
  description = "Base64 cluster CA certificate."
  value       = aws_eks_cluster.this.certificate_authority[0].data
}

output "region" {
  description = "AWS region the cluster runs in."
  value       = var.region
}

output "node_group" {
  description = "Managed node group name."
  value       = aws_eks_node_group.this.node_group_name
}
