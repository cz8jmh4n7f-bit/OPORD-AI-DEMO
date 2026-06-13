output "resource_group_name" {
  value       = azurerm_resource_group.this.name
  description = "Resource group that owns the cluster."
}

output "cluster_name" {
  value       = azurerm_kubernetes_cluster.this.name
  description = "AKS cluster name."
}

output "cluster_id" {
  value       = azurerm_kubernetes_cluster.this.id
  description = "Full ARM resource ID."
}

output "fqdn" {
  value       = azurerm_kubernetes_cluster.this.fqdn
  description = "API server FQDN."
}

output "kube_config" {
  value       = azurerm_kubernetes_cluster.this.kube_config_raw
  description = "Raw kubeconfig (sensitive). OPORD writes this to disk so kubectl works without `az aks get-credentials`."
  sensitive   = true
}

# Convenience: the equivalent `az` command, for users who prefer the canonical flow.
output "kubeconfig_az_command" {
  value       = "az aks get-credentials --resource-group ${azurerm_resource_group.this.name} --name ${azurerm_kubernetes_cluster.this.name}"
  description = "Equivalent az CLI command to fetch kubeconfig manually."
}
