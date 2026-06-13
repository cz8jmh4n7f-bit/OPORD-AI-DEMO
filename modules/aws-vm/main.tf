locals {
  names = [for i in range(var.vm_count) : format("%s-%02d", var.name_prefix, i + 1)]
}

resource "aws_instance" "vm" {
  count = var.vm_count

  ami           = var.ami
  instance_type = var.instance_type

  subnet_id              = var.subnet_id != "" ? var.subnet_id : null
  vpc_security_group_ids = length(var.security_group_ids) > 0 ? var.security_group_ids : null
  key_name               = var.key_name != "" ? var.key_name : null
  # Only force a public IP when asked; otherwise leave it null so the subnet's
  # own map_public_ip_on_launch decides. Setting `false` explicitly on a default
  # subnet (which auto-assigns) creates a perpetual ForceNew diff to the instance
  # gets replaced on every re-apply (e.g. on scale). null avoids that.
  associate_public_ip_address = var.associate_public_ip ? true : null

  root_block_device {
    volume_size = var.root_volume_gb
    volume_type = "gp3"
  }

  tags = {
    Name        = local.names[count.index]
    Environment = var.environment
    ManagedBy   = "opord"
  }
}
