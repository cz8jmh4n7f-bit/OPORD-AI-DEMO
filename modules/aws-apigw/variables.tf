variable "name" {
  description = "Name of the API Gateway HTTP API."
  type        = string
}

variable "integration_type" {
  description = "Backend integration type: 'lambda' (AWS_PROXY) or 'http' (HTTP_PROXY)."
  type        = string
  default     = "lambda"
}

variable "integration_target" {
  description = "Integration target: a Lambda function ARN when integration_type='lambda', or an upstream URL when 'http'. Empty leaves the route unintegrated."
  type        = string
  default     = ""
}

variable "route_key" {
  description = "Route key for the API (e.g. '$default' or 'GET /items')."
  type        = string
  default     = "$default"
}

variable "domain_name" {
  description = "Optional custom domain name. When set, a domain + API mapping (and a Route53 ALIAS if hosted_zone_id is given) are created."
  type        = string
  default     = ""
}

variable "certificate_arn" {
  description = "ACM certificate ARN for the custom domain (required when domain_name is set; the cert must be in the API's region)."
  type        = string
  default     = ""
}

variable "hosted_zone_id" {
  description = "Route53 hosted zone id for the custom domain. When set (with domain_name), an ALIAS record pointing at the API Gateway domain is created."
  type        = string
  default     = ""
}

variable "region" {
  description = "AWS region. Empty defers to the ambient AWS_REGION that OPORD injects."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags applied to taggable resources."
  type        = map(string)
  default     = {}
}
