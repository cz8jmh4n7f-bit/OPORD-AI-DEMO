locals {
  base_tags = merge(var.tags, { Name = var.name, ManagedBy = "opord" })
}

resource "aws_secretsmanager_secret" "this" {
  name                    = var.name
  description             = var.description
  kms_key_id              = var.kms_key_arn == "" ? null : var.kms_key_arn
  recovery_window_in_days = var.recovery_window_days
  tags                    = local.base_tags
}

# Optional: automatic rotation via a customer-provided Lambda.
resource "aws_secretsmanager_secret_rotation" "this" {
  count               = var.rotation_lambda_arn == "" ? 0 : 1
  secret_id           = aws_secretsmanager_secret.this.id
  rotation_lambda_arn = var.rotation_lambda_arn
  rotation_rules {
    automatically_after_days = var.rotation_days
  }
}
