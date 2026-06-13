output "vm_names" {
  description = "Name tags of the created instances."
  value       = local.names
}

output "vm_ids" {
  description = "EC2 instance IDs."
  value       = aws_instance.vm[*].id
}

output "private_ips" {
  description = "Private IPs."
  value       = aws_instance.vm[*].private_ip
}

output "public_ips" {
  description = "Public IPs (empty unless associate_public_ip)."
  value       = aws_instance.vm[*].public_ip
}
