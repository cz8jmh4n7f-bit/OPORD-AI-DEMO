# OPORD module - serverless function (AWS Lambda). Generic "function" blueprint.
# Auto-creates the execution IAM role. With no S3 code given, ships a minimal
# built-in python "hello" handler so the function is immediately invokable.
# Credentials come from the ambient environment (AWS_ACCESS_KEY_ID/SECRET) that
# OPORD injects from the resolved provider creds (OpenBao/env).

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    archive = {
      source  = "hashicorp/archive"
      version = ">= 2.4"
    }
  }

  backend "pg" {}
}

provider "aws" {
  region = var.region
}
