locals {
  base_tags = merge(var.tags, {
    Environment = var.environment
    ManagedBy   = "opord"
  })
  raw_name = lower(replace(var.name_prefix, "/[^a-z0-9-]/", ""))
  # Service Bus namespace names cannot end with -sb, -mgmt, or a hyphen, and
  # must be 6-50 chars. Strip trailing -sb if the caller already supplied it,
  # then append -ns; the global-uniqueness check happens at apply time.
  base_name      = substr(trimsuffix(local.raw_name, "-sb"), 0, 40)
  suffix         = substr(md5(var.name_prefix), 0, 6)
  namespace_name = "${local.base_name}-${local.suffix}-ns"
}

resource "azurerm_resource_group" "this" {
  name     = "${var.name_prefix}-rg"
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_servicebus_namespace" "this" {
  name                = local.namespace_name
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  sku                 = var.sku
  tags                = local.base_tags
}

resource "azurerm_servicebus_queue" "this" {
  count        = length(var.queue_names)
  name         = var.queue_names[count.index]
  namespace_id = azurerm_servicebus_namespace.this.id

  max_size_in_megabytes                = var.max_size_megabytes
  lock_duration                        = var.lock_duration
  dead_lettering_on_message_expiration = var.enable_dead_lettering
}
