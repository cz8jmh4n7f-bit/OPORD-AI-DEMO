locals {
  provisioned = var.billing_mode == "PROVISIONED"
  has_range   = var.range_key != ""
}

resource "aws_dynamodb_table" "this" {
  name         = var.name
  billing_mode = var.billing_mode
  hash_key     = var.hash_key
  range_key    = local.has_range ? var.range_key : null

  # Capacity applies only to PROVISIONED; null lets on-demand tables omit it
  # (DynamoDB rejects capacity values when billing_mode is PAY_PER_REQUEST).
  read_capacity  = local.provisioned ? var.read_capacity : null
  write_capacity = local.provisioned ? var.write_capacity : null

  attribute {
    name = var.hash_key
    type = var.hash_key_type
  }

  dynamic "attribute" {
    for_each = local.has_range ? [1] : []
    content {
      name = var.range_key
      type = var.range_key_type
    }
  }

  tags = {
    Name        = var.name
    Environment = var.environment
    ManagedBy   = "opord"
  }
}
