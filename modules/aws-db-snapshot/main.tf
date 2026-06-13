variable "region" {
  type        = string
  description = "AWS region."
}

variable "db_identifier" {
  type        = string
  description = "Identifier of the source RDS instance to snapshot."
}

variable "snapshot_name" {
  type        = string
  description = "Identifier for the new snapshot."
}

resource "aws_db_snapshot" "this" {
  db_instance_identifier = var.db_identifier
  db_snapshot_identifier = var.snapshot_name
}

output "snapshot_id" {
  description = "The created DB snapshot identifier."
  value       = aws_db_snapshot.this.db_snapshot_identifier
}

output "snapshot_arn" {
  description = "The created DB snapshot ARN."
  value       = aws_db_snapshot.this.db_snapshot_arn
}
