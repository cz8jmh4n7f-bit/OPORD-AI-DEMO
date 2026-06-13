locals {
  base_tags = merge(var.tags, {
    Project   = var.name_prefix
    CsaId     = var.csa_id
    ManagedBy = "opord"
  })
  vnet_name = "${var.name_prefix}-${var.csa_id}-vnet"
  nsg_name  = "${var.name_prefix}-${var.csa_id}-nsg"
  # Carve three /24s out of the /22.
  subnet_cidrs = [
    cidrsubnet(var.vnet_cidr, 2, 0),
    cidrsubnet(var.vnet_cidr, 2, 1),
    cidrsubnet(var.vnet_cidr, 2, 2),
  ]
}

resource "azurerm_virtual_network" "this" {
  name                = local.vnet_name
  location            = var.location
  resource_group_name = var.network_rg_name
  address_space       = [var.vnet_cidr]
  tags                = local.base_tags
}

resource "azurerm_subnet" "this" {
  count                = 3
  name                 = "${var.name_prefix}-${var.csa_id}-subnet-${count.index + 1}"
  resource_group_name  = var.network_rg_name
  virtual_network_name = azurerm_virtual_network.this.name
  address_prefixes     = [local.subnet_cidrs[count.index]]
}

# NSG with ZTNA semantics: explicit allow for SSH/RDP/HTTPS/ICMP from trusted
# CIDRs, deny-all otherwise. Default NSG rules already deny inbound from
# Internet at priority 65500, but pinning an explicit 4096 priority deny gives
# auditors a single rule they can point at.
resource "azurerm_network_security_group" "this" {
  name                = local.nsg_name
  location            = var.location
  resource_group_name = var.network_rg_name
  tags                = local.base_tags

  security_rule {
    name                       = "allow-ssh"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefixes    = var.allow_inbound_cidrs
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-rdp"
    priority                   = 110
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "3389"
    source_address_prefixes    = var.allow_inbound_cidrs
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-https"
    priority                   = 120
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "443"
    source_address_prefixes    = var.allow_inbound_cidrs
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-icmp"
    priority                   = 130
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Icmp"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefixes    = var.allow_inbound_cidrs
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "deny-all-inbound"
    priority                   = 4096
    direction                  = "Inbound"
    access                     = "Deny"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
}

# Bind the NSG to each subnet. Per-subnet bind (vs VNet-wide) means future
# subnets can opt in/out individually.
resource "azurerm_subnet_network_security_group_association" "this" {
  count                     = 3
  subnet_id                 = azurerm_subnet.this[count.index].id
  network_security_group_id = azurerm_network_security_group.this.id
}

# Network Watcher per region is a tenant-level singleton; reuse the
# auto-provisioned one (azurerm creates it on demand for Flow Logs). Only
# looked up when Flow Logs are enabled - the data source read otherwise runs
# on every plan/destroy and would fail if the region has no Network Watcher.
data "azurerm_network_watcher" "this" {
  count               = var.create_flow_logs ? 1 : 0
  name                = "NetworkWatcher_${var.location}"
  resource_group_name = "NetworkWatcherRG"
}

# NSG Flow Logs were retired by Azure on 2025-06-30 (full removal 2027-09-30);
# the replacement is VNet Flow Logs which capture richer data and target the
# VNet directly. azurerm v4 uses the same resource - only target_resource_id
# changes from NSG to VNet. An active Flow Log blocks the VNet (and so the
# network RG) from deleting, so destroy must remove it cleanly - hence the
# real storage_account_id is passed on destroy too (see DestroyAccount L4).
resource "azurerm_network_watcher_flow_log" "this" {
  count                = var.create_flow_logs ? 1 : 0
  network_watcher_name = data.azurerm_network_watcher.this[0].name
  resource_group_name  = data.azurerm_network_watcher.this[0].resource_group_name
  name                 = "${var.name_prefix}-${var.csa_id}-flow-log"

  target_resource_id = azurerm_virtual_network.this.id
  storage_account_id = var.flow_logs_storage_account_id
  enabled            = true

  retention_policy {
    enabled = true
    days    = var.flow_logs_retention_days
  }

  version = 2
  tags    = local.base_tags
}
