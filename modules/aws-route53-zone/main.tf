locals {
  tags = merge({
    ManagedBy = "opord"
    Name      = var.name
  }, var.tags)

  # Key records by name+type so for_each is stable across plans.
  records = { for r in var.records : "${r.name}-${r.type}" => r }
}

resource "aws_route53_zone" "this" {
  name = var.name
  tags = local.tags

  # A vpc {} block is only present for a private zone; its presence is what
  # makes the zone private, so it is gated with a dynamic block.
  dynamic "vpc" {
    for_each = var.private ? [1] : []
    content {
      vpc_id = var.vpc_id
    }
  }
}

resource "aws_route53_record" "this" {
  for_each = local.records

  zone_id = aws_route53_zone.this.zone_id
  name    = each.value.name
  type    = each.value.type

  # NOTE (v1): true ALIAS records need a target hosted-zone-id (e.g. an ALB or
  # CloudFront zone) which this module does not yet thread through. Even when a
  # record is flagged alias = true we emit a plain value record with a TTL so the
  # module stays valid and useful. A later version will accept a target zone id
  # and switch to an alias {} block when present.
  records = [each.value.value]
  ttl     = coalesce(each.value.ttl, 300)
}
