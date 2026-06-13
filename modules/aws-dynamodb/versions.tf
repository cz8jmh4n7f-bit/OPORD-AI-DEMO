# OPORD module - managed NoSQL table (AWS DynamoDB). Generic "table" blueprint.
# Credentials come from the ambient environment (AWS_ACCESS_KEY_ID/SECRET) which
# OPORD injects from the resolved provider creds (OpenBao/env).

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
