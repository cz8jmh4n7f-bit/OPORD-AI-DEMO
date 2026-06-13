locals {
  base_tags = merge(var.tags, {
    Environment = var.environment
    ManagedBy   = "opord"
  })
  cluster_name = "${var.name_prefix}-aks"
  dns_prefix   = var.dns_prefix != "" ? var.dns_prefix : substr(replace(var.name_prefix, "/[^a-z0-9]/", ""), 0, 50)
}

resource "azurerm_resource_group" "this" {
  name     = "${var.name_prefix}-rg"
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_kubernetes_cluster" "this" {
  name                = local.cluster_name
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  dns_prefix          = local.dns_prefix
  kubernetes_version  = var.kubernetes_version != "" ? var.kubernetes_version : null
  tags                = local.base_tags

  default_node_pool {
    name            = "system"
    node_count      = var.node_count
    vm_size         = var.node_vm_size
    os_disk_size_gb = var.node_disk_gb
    type            = "VirtualMachineScaleSets"
  }

  # SystemAssigned identity = AKS manages its own service principal.
  # No extra IAM plumbing needed (the AKS RP is granted the right perms by Azure).
  identity {
    type = "SystemAssigned"
  }

  network_profile {
    network_plugin = "azure"
    load_balancer_sku = "standard"
  }
}
