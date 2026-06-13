# OPORD first-class module: ElastiCache Redis replication group with secure
# defaults (at-rest + in-transit encryption, private subnet group, VPC-scoped
# security group). For sessions, rate-limiting, application caching.
# Stack-style - NO backend block; OPORD injects a workspace pg backend.
# Cost: from ~$12/month for cache.t4g.micro single-node; scales with node type.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}
