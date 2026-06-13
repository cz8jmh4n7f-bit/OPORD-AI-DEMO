output "subscription_id" {
  value = var.mode == "adopt" ? (
    length(data.azurerm_subscription.adopted) > 0 ? data.azurerm_subscription.adopted[0].subscription_id : ""
  ) : (
    length(azurerm_subscription.created) > 0 ? azurerm_subscription.created[0].subscription_id : ""
  )
  description = "The subscription GUID downstream layers operate on."
}

output "subscription_name" {
  value = var.mode == "adopt" ? (
    length(data.azurerm_subscription.adopted) > 0 ? data.azurerm_subscription.adopted[0].display_name : ""
  ) : (
    length(azurerm_subscription.created) > 0 ? azurerm_subscription.created[0].subscription_name : local.derived_name
  )
  description = "Subscription display name."
}

output "tenant_id" {
  value = var.mode == "adopt" ? (
    length(data.azurerm_subscription.adopted) > 0 ? data.azurerm_subscription.adopted[0].tenant_id : ""
  ) : ""
  description = "Tenant GUID the subscription belongs to (adopt-mode only - create-mode infers it from the SP)."
}

output "subscription_resource_id" {
  value = var.mode == "create" ? (
    length(azurerm_subscription.created) > 0 ? azurerm_subscription.created[0].id : ""
  ) : "/subscriptions/${var.subscription_id}"
  description = "Full ARM resource ID for the subscription (use as scope for role assignments)."
}

output "mode" {
  value       = var.mode
  description = "Which path L1 executed (adopt or create). Useful for downstream conditional logic + audit."
}
