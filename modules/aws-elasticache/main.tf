locals {
  base_tags = merge(var.tags, { Name = var.name, ManagedBy = "opord" })
  # Default parameter group per major (Redis 7.x). Override via var if needed.
  default_pg = "default.redis7"
}

# Pull the VPC + CIDR from the first subnet so the auto-SG opens 6379 only to
# in-VPC clients. If the caller supplies their own SG, neither lookup nor SG
# is needed.
data "aws_subnet" "first" {
  count = length(var.security_group_ids) == 0 ? 1 : 0
  id    = var.subnet_ids[0]
}

resource "aws_elasticache_subnet_group" "this" {
  name       = "${var.name}-subnet"
  subnet_ids = var.subnet_ids
  tags       = local.base_tags
}

resource "aws_security_group" "this" {
  count       = length(var.security_group_ids) == 0 ? 1 : 0
  name        = "${var.name}-redis-sg"
  description = "Redis 6379 from VPC for ${var.name}"
  vpc_id      = data.aws_subnet.first[0].vpc_id

  ingress {
    description = "Redis from VPC CIDR"
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    cidr_blocks = [data.aws_subnet.first[0].cidr_block]
  }

  egress {
    description = "All outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = local.base_tags
}

resource "aws_elasticache_replication_group" "this" {
  replication_group_id = var.name
  description          = "Redis ${var.name} (OPORD)"

  engine               = "redis"
  engine_version       = var.engine_version
  node_type            = var.node_type
  num_cache_clusters   = var.num_cache_nodes
  parameter_group_name = var.parameter_group_name == "" ? local.default_pg : var.parameter_group_name
  port                 = 6379

  subnet_group_name = aws_elasticache_subnet_group.this.name
  security_group_ids = length(var.security_group_ids) > 0 ? var.security_group_ids : [
    aws_security_group.this[0].id,
  ]

  at_rest_encryption_enabled = var.at_rest_encryption
  transit_encryption_enabled = var.in_transit_encryption
  auth_token                 = var.auth_token == "" ? null : var.auth_token

  # Multi-node sets are HA: enable Multi-AZ + automatic failover.
  automatic_failover_enabled = var.num_cache_nodes > 1
  multi_az_enabled           = var.num_cache_nodes > 1

  tags = local.base_tags
}
