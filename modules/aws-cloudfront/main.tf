# CloudFront distribution fronting a single origin. v1 keeps the simple, always-
# valid path: a custom origin config (S3 website endpoints work as custom origins;
# S3 REST + OAC is a v2 enhancement), forwarded_values (cache policies are a v2
# enhancement), and a default-cert-or-ACM viewer certificate switch. CloudFront is
# global; create/destroy is slow (~10-20 min) so this runs on the durable River path.

resource "aws_cloudfront_distribution" "this" {
  enabled             = true
  comment             = var.name
  aliases             = var.aliases
  price_class         = var.price_class
  default_root_object = var.default_root_object != "" ? var.default_root_object : null

  origin {
    domain_name = var.origin_domain
    origin_id   = "opord-origin"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = var.origin_type == "s3" ? "http-only" : "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "opord-origin"
    viewer_protocol_policy = "redirect-to-https"

    forwarded_values {
      query_string = true

      cookies {
        forward = "none"
      }
    }
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  # Default *.cloudfront.net cert when no ACM cert is supplied; otherwise the
  # caller-provided us-east-1 ACM cert with SNI.
  dynamic "viewer_certificate" {
    for_each = var.certificate_arn != "" ? [1] : []
    content {
      acm_certificate_arn      = var.certificate_arn
      ssl_support_method       = "sni-only"
      minimum_protocol_version = "TLSv1.2_2021"
    }
  }

  dynamic "viewer_certificate" {
    for_each = var.certificate_arn == "" ? [1] : []
    content {
      cloudfront_default_certificate = true
    }
  }

  tags = var.tags
}
