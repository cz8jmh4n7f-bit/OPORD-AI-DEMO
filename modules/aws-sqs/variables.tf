variable "region" {
  type        = string
  description = "AWS region."
}

variable "name" {
  type        = string
  description = "Queue base name. For FIFO the module appends '.fifo' automatically."
}

variable "fifo" {
  type        = bool
  description = "FIFO queue (ordered, exactly-once). Standard otherwise."
  default     = false
}

variable "visibility_timeout_seconds" {
  type        = number
  description = "How long a message is invisible after being received (0-43200)."
  default     = 30
}

variable "message_retention_seconds" {
  type        = number
  description = "How long unconsumed messages survive (60-1209600). Default 4 days."
  default     = 345600
}

variable "max_message_size_bytes" {
  type        = number
  description = "Per-message size limit (1024-262144). Default 256 KB (max)."
  default     = 262144
}

variable "receive_wait_time_seconds" {
  type        = number
  description = "Long-polling wait (0-20). Set > 0 to reduce empty-receive cost."
  default     = 0
}

variable "dlq_enabled" {
  type        = bool
  description = "Auto-create a sibling dead-letter queue + redrive policy."
  default     = false
}

variable "dlq_max_receive_count" {
  type        = number
  description = "Messages move to DLQ after N failed receives. Ignored if dlq_enabled=false."
  default     = 5
}

variable "kms_key_arn" {
  type        = string
  description = "Customer-managed KMS key ARN. Empty = AWS-managed SQS encryption (free)."
  default     = ""
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto both queues."
  default     = {}
}
