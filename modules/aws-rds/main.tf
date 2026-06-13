locals {
  tags = {
    Environment = var.environment
    ManagedBy   = "opord"
    Name        = var.name
  }
  # A DB subnet group is only created when subnets are supplied; otherwise RDS
  # uses the default VPC's default subnet group.
  use_subnet_group = length(var.subnet_ids) > 0
}

resource "aws_db_subnet_group" "this" {
  count      = local.use_subnet_group ? 1 : 0
  name       = "${var.name}-subnets"
  subnet_ids = var.subnet_ids
  tags       = local.tags
}

resource "aws_db_instance" "this" {
  identifier     = var.name
  engine         = var.engine
  engine_version = var.engine_version != "" ? var.engine_version : null
  instance_class = var.instance_class

  allocated_storage = var.storage_gb
  db_name           = var.db_name
  username          = var.username

  # OPORD never handles a plaintext master password; RDS stores it in Secrets Manager.
  manage_master_user_password = true

  db_subnet_group_name   = local.use_subnet_group ? aws_db_subnet_group.this[0].name : null
  vpc_security_group_ids = length(var.security_group_ids) > 0 ? var.security_group_ids : null

  multi_az            = var.multi_az
  publicly_accessible = var.public_access
  skip_final_snapshot = true
  deletion_protection = false

  tags = local.tags
}
