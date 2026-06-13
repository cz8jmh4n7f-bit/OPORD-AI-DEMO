variable "name" {
  type        = string
  description = "Topic name (the queue). A pull subscription '<name>-sub' is created."
}

variable "ack_deadline_seconds" {
  type    = number
  default = 30
}

variable "message_retention_seconds" {
  type    = number
  default = 604800 # 7 days (Pub/Sub max for a subscription)
}

variable "dlq_enabled" {
  type    = bool
  default = false
}

variable "dlq_max_delivery_attempts" {
  type    = number
  default = 5 # Pub/Sub requires 5..100
}

variable "labels" {
  type    = map(string)
  default = {}
}

locals {
  ack_deadline   = max(10, min(600, var.ack_deadline_seconds))
  retention      = max(600, min(604800, var.message_retention_seconds))
  max_deliveries = max(5, min(100, var.dlq_max_delivery_attempts))
}

resource "google_pubsub_topic" "this" {
  name   = var.name
  labels = var.labels
}

resource "google_pubsub_topic" "dlq" {
  count  = var.dlq_enabled ? 1 : 0
  name   = "${var.name}-dlq"
  labels = var.labels
}

resource "google_pubsub_subscription" "this" {
  name                       = "${var.name}-sub"
  topic                      = google_pubsub_topic.this.id
  ack_deadline_seconds       = local.ack_deadline
  message_retention_duration = "${local.retention}s"
  labels                     = var.labels

  dynamic "dead_letter_policy" {
    for_each = var.dlq_enabled ? [1] : []
    content {
      dead_letter_topic     = google_pubsub_topic.dlq[0].id
      max_delivery_attempts = local.max_deliveries
    }
  }
}

output "queue_url" {
  value       = google_pubsub_topic.this.id
  description = "The topic resource id (projects/.../topics/...)."
}

output "queue_arn" {
  value = google_pubsub_topic.this.id
}

output "name" {
  value = google_pubsub_topic.this.name
}

output "dlq_url" {
  value       = var.dlq_enabled ? google_pubsub_topic.dlq[0].id : ""
  description = "The dead-letter topic id, if enabled."
}

output "subscription" {
  value = google_pubsub_subscription.this.id
}
