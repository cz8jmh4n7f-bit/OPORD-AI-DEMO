output "domain_name" {
  description = "The *.cloudfront.net domain name of the distribution."
  value       = aws_cloudfront_distribution.this.domain_name
}

output "distribution_id" {
  description = "The CloudFront distribution id."
  value       = aws_cloudfront_distribution.this.id
}

output "arn" {
  description = "The ARN of the CloudFront distribution."
  value       = aws_cloudfront_distribution.this.arn
}

output "hosted_zone_id" {
  description = "The fixed CloudFront Route53 hosted zone id (for an ALIAS record pointing at the distribution)."
  value       = aws_cloudfront_distribution.this.hosted_zone_id
}
