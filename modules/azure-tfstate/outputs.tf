output "resource_group_name" {
  value       = azurerm_resource_group.this.name
  description = "Resource group that owns the tfstate storage account."
}

output "storage_account_name" {
  value       = azurerm_storage_account.this.name
  description = "Storage account name (use as `storage_account_name` in tofu's azurerm backend block)."
}

output "container_name" {
  value       = azurerm_storage_container.tfstate.name
  description = "Blob container name (use as `container_name` in the azurerm backend block)."
}

output "backend_config_example" {
  value = <<-EOT
    terraform {
      backend "azurerm" {
        resource_group_name  = "${azurerm_resource_group.this.name}"
        storage_account_name = "${azurerm_storage_account.this.name}"
        container_name       = "${azurerm_storage_container.tfstate.name}"
        key                  = "<unique-state-name>.tfstate"
        # State locking uses native Azure blob lease - no DynamoDB-equivalent
        # needed. Authentication: AZURE_TENANT_ID / CLIENT_ID / CLIENT_SECRET
        # picked up from environment (same as the azurerm provider).
      }
    }
  EOT
  description = "Drop-in backend block. Each module gets its own `key` so state files don't collide."
}
