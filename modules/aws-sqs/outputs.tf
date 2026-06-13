output "queue_url" {
  description = "Main queue URL (use with SDK SendMessage/ReceiveMessage)."
  value       = aws_sqs_queue.this.url
}

output "queue_arn" {
  description = "Main queue ARN (use in IAM policies + event source mappings)."
  value       = aws_sqs_queue.this.arn
}

output "queue_name" {
  description = "Main queue name as AWS sees it (with .fifo suffix if FIFO)."
  value       = aws_sqs_queue.this.name
}

output "dlq_url" {
  description = "Dead-letter queue URL (null when dlq_enabled=false)."
  value       = var.dlq_enabled ? aws_sqs_queue.dlq[0].url : null
}

output "dlq_arn" {
  description = "Dead-letter queue ARN (null when dlq_enabled=false)."
  value       = var.dlq_enabled ? aws_sqs_queue.dlq[0].arn : null
}
