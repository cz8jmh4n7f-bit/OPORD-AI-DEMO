locals {
  base_tags = merge(var.tags, {
    Environment = var.environment
    ManagedBy   = "opord"
  })
  vm_names = [for i in range(var.vm_count) : format("%s-%02d", var.name_prefix, i + 1)]
}

# Every OPORD-provisioned set lives in its own resource group: clean blast
# radius, single-click delete for ops.
resource "azurerm_resource_group" "this" {
  name     = "${var.name_prefix}-rg"
  location = var.location
  tags     = local.base_tags
}

# Auto-create a small VNet + subnet when the caller did not supply one.
resource "azurerm_virtual_network" "this" {
  count               = var.subnet_id == "" ? 1 : 0
  name                = "${var.name_prefix}-vnet"
  address_space       = [var.vnet_cidr]
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  tags                = local.base_tags
}

resource "azurerm_subnet" "this" {
  count                = var.subnet_id == "" ? 1 : 0
  name                 = "${var.name_prefix}-subnet"
  resource_group_name  = azurerm_resource_group.this.name
  virtual_network_name = azurerm_virtual_network.this[0].name
  address_prefixes     = [var.vnet_cidr]
}

# Locked-down NSG: SSH from allow_ssh_from, all other inbound denied (default).
resource "azurerm_network_security_group" "this" {
  name                = "${var.name_prefix}-nsg"
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  tags                = local.base_tags

  security_rule {
    name                       = "allow-ssh"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefixes    = var.allow_ssh_from
    destination_address_prefix = "*"
  }
}

# Public IPs per-VM (only when associate_public_ip).
resource "azurerm_public_ip" "this" {
  count               = var.associate_public_ip ? var.vm_count : 0
  name                = "${local.vm_names[count.index]}-pip"
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  allocation_method   = "Static"
  sku                 = "Standard"
  tags                = local.base_tags
}

resource "azurerm_network_interface" "this" {
  count               = var.vm_count
  name                = "${local.vm_names[count.index]}-nic"
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  tags                = local.base_tags

  ip_configuration {
    name                          = "primary"
    subnet_id                     = var.subnet_id != "" ? var.subnet_id : azurerm_subnet.this[0].id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = var.associate_public_ip ? azurerm_public_ip.this[count.index].id : null
  }
}

resource "azurerm_network_interface_security_group_association" "this" {
  count                     = var.vm_count
  network_interface_id      = azurerm_network_interface.this[count.index].id
  network_security_group_id = azurerm_network_security_group.this.id
}

resource "azurerm_linux_virtual_machine" "this" {
  count                           = var.vm_count
  name                            = local.vm_names[count.index]
  resource_group_name             = azurerm_resource_group.this.name
  location                        = azurerm_resource_group.this.location
  size                            = var.vm_size
  admin_username                  = var.admin_username
  disable_password_authentication = true
  network_interface_ids           = [azurerm_network_interface.this[count.index].id]
  tags                            = local.base_tags

  admin_ssh_key {
    username   = var.admin_username
    public_key = var.ssh_public_key
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
    disk_size_gb         = var.os_disk_gb
  }

  source_image_reference {
    publisher = var.image.publisher
    offer     = var.image.offer
    sku       = var.image.sku
    version   = var.image.version
  }
}
