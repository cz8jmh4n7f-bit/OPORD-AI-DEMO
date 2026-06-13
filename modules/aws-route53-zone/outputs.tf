output "zone_id" {
  description = "The Route53 hosted zone id."
  value       = aws_route53_zone.this.zone_id
}

output "zone_name" {
  description = "The hosted zone domain name."
  value       = aws_route53_zone.this.name
}

output "name_servers" {
  description = "The authoritative name servers for the zone (delegate these at the registrar for a public zone)."
  value       = aws_route53_zone.this.name_servers
}
