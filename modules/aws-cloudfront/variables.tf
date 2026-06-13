variable "name" {
  description = "Name of the distribution (used as the CloudFront comment)."
  type        = string
}

variable "origin_type" {
  description = "Origin kind: 's3' | 'alb' | 'apigw' | 'custom'. Drives the origin protocol policy (s3 website endpoints are http-only; everything else is https-only)."
  type        = string
  default     = "s3"
}

variable "origin_domain" {
  description = "The origin domain name CloudFront fetches from: an S3 website/regional domain, an ALB DNS name, an API Gateway domain, or a custom origin host."
  type        = string
}

variable "aliases" {
  description = "Alternate domain names (CNAMEs) served by the distribution. Each alias requires a matching us-east-1 ACM certificate (certificate_arn)."
  type        = list(string)
  default     = []
}

variable "certificate_arn" {
  description = "ACM certificate ARN for the aliases. MUST be a us-east-1 cert when set (the caller guarantees the region). Empty uses the default *.cloudfront.net certificate."
  type        = string
  default     = ""
}

variable "default_root_object" {
  description = "Object returned for a request to the root URL (e.g. 'index.html'). Empty leaves it unset."
  type        = string
  default     = ""
}

variable "price_class" {
  description = "CloudFront edge-location price class: 'PriceClass_100' | 'PriceClass_200' | 'PriceClass_All'."
  type        = string
  default     = "PriceClass_100"
}

variable "region" {
  description = "AWS provider region. Empty defers to the ambient AWS_REGION that OPORD injects. (CloudFront is global; this only sets the provider.)"
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags applied to the distribution."
  type        = map(string)
  default     = {}
}
