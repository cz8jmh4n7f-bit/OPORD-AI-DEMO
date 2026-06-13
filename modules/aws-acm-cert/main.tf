locals {
  tags = merge({
    ManagedBy = "opord"
    Name      = var.domain
  }, var.tags)

  # Automatic Route53 DNS validation only happens when a hosted-zone id is given.
  do_validation = var.validation_zone_id != ""
}

resource "aws_acm_certificate" "this" {
  domain_name               = var.domain
  subject_alternative_names = var.subject_alternative_names
  validation_method         = "DNS"

  tags = local.tags

  lifecycle {
    create_before_destroy = true
  }
}

# Validation records in Route53 - one per domain/SAN. Absent when no zone is given.
resource "aws_route53_record" "validation" {
  for_each = local.do_validation ? {
    for dvo in aws_acm_certificate.this.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      type   = dvo.resource_record_type
      record = dvo.resource_record_value
    }
  } : {}

  allow_overwrite = true
  name            = each.value.name
  type            = each.value.type
  records         = [each.value.record]
  ttl             = 60
  zone_id         = var.validation_zone_id
}

# Waits for the certificate to be validated. Absent when no zone is given.
resource "aws_acm_certificate_validation" "this" {
  count = local.do_validation ? 1 : 0

  certificate_arn         = aws_acm_certificate.this.arn
  validation_record_fqdns = [for r in aws_route53_record.validation : r.fqdn]

  timeouts {
    create = "45m"
  }
}
