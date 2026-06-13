variable "region" {
  type        = string
  description = "AWS region, e.g. eu-central-1."
}

variable "name" {
  type        = string
  description = "Lambda function name."
}

variable "runtime" {
  type        = string
  description = "Lambda runtime (e.g. python3.12, nodejs20.x). The built-in handler is python - set s3 code for other runtimes."
  default     = "python3.12"
}

variable "handler" {
  type        = string
  description = "Function entry point (e.g. index.handler)."
  default     = "index.handler"
}

variable "memory_mb" {
  type        = number
  description = "Memory in MB."
  default     = 128
}

variable "timeout_sec" {
  type        = number
  description = "Timeout in seconds."
  default     = 10
}

variable "s3_bucket" {
  type        = string
  description = "S3 bucket holding the deployment zip (empty = use the built-in handler)."
  default     = ""
}

variable "s3_key" {
  type        = string
  description = "S3 key of the deployment zip."
  default     = ""
}

variable "env_vars" {
  type        = map(string)
  description = "Environment variables for the function."
  default     = {}
}

variable "environment" {
  type        = string
  description = "Environment label (dev, test, production)."
  default     = "dev"
}
