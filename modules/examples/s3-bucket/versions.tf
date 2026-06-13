# Example OPORD "stack" module: an S3 bucket. Demonstrates the generic
# stack escape hatch - any OpenTofu root module works as long as it does NOT
# declare a backend (OPORD injects a workspace-isolated pg backend).

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
