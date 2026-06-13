output "vm_names" {
  value = google_compute_instance.vm[*].name
}

output "vm_ids" {
  value = google_compute_instance.vm[*].id
}

output "private_ips" {
  value = google_compute_instance.vm[*].network_interface[0].network_ip
}

output "public_ips" {
  value = [for vm in google_compute_instance.vm : try(vm.network_interface[0].access_config[0].nat_ip, "")]
}
