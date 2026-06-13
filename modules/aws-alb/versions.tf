# OPORD original module - AWS expose-layer Application Load Balancer (ADR-0016).
# Creates an internet-facing or internal ALB with listener(s) and a target group
# in an existing VPC. When no security group is supplied, a VPC-CIDR-scoped SG is
# auto-created opening the listener ports. AWS credentials come from the ambient
# environment.

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }

  backend "pg" {}
}

provider "aws" {
  region = var.region
}
