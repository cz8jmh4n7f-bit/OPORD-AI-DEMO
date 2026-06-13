locals {
  base_tags = merge(var.tags, { ManagedBy = "opord" })
  suffix    = var.fifo ? ".fifo" : ""
  full_name = "${var.name}${local.suffix}"
  dlq_name  = "${var.name}-dlq${local.suffix}"
}

# Sibling DLQ created when dlq_enabled. Has its own 14-day retention so failed
# messages survive long enough for ops to inspect.
resource "aws_sqs_queue" "dlq" {
  count                       = var.dlq_enabled ? 1 : 0
  name                        = local.dlq_name
  fifo_queue                  = var.fifo
  content_based_deduplication = var.fifo
  message_retention_seconds   = 1209600 # 14 days

  sqs_managed_sse_enabled = var.kms_key_arn == ""
  kms_master_key_id       = var.kms_key_arn == "" ? null : var.kms_key_arn

  tags = merge(local.base_tags, { Name = local.dlq_name, Role = "dlq" })
}

resource "aws_sqs_queue" "this" {
  name                        = local.full_name
  fifo_queue                  = var.fifo
  content_based_deduplication = var.fifo

  visibility_timeout_seconds = var.visibility_timeout_seconds
  message_retention_seconds  = var.message_retention_seconds
  max_message_size           = var.max_message_size_bytes
  receive_wait_time_seconds  = var.receive_wait_time_seconds

  sqs_managed_sse_enabled = var.kms_key_arn == ""
  kms_master_key_id       = var.kms_key_arn == "" ? null : var.kms_key_arn

  redrive_policy = var.dlq_enabled ? jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq[0].arn
    maxReceiveCount     = var.dlq_max_receive_count
  }) : null

  tags = merge(local.base_tags, { Name = local.full_name })
}
