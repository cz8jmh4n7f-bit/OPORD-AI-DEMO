locals {
  base_tags = merge(var.tags, { Name = var.name, ManagedBy = "opord" })

  is_lambda = var.target_type == "lambda"

  # Auto-create a SG only when the caller supplies none.
  make_sg = length(var.security_group_ids) == 0
  sg_ids  = length(var.security_group_ids) > 0 ? var.security_group_ids : [aws_security_group.this[0].id]

  # Ingress source: anywhere for internet-facing, the VPC CIDR for internal.
  ingress_cidr = var.internal ? [data.aws_subnet.first.cidr_block] : ["0.0.0.0/0"]

  # One target group exists either way; pick whichever is active.
  target_group_arn = local.is_lambda ? aws_lb_target_group.lambda[0].arn : aws_lb_target_group.ip[0].arn

  # Register targets only for instance/ip (lambda registers its ARN separately).
  attach_targets = local.is_lambda ? [] : var.targets
}

# Pull the VPC + CIDR from the first subnet: the auto-SG and the target group
# both need the VPC id, and the SG ingress is scoped to the VPC CIDR for internal.
data "aws_subnet" "first" {
  id = var.subnet_ids[0]
}

resource "aws_security_group" "this" {
  count       = local.make_sg ? 1 : 0
  name        = "${var.name}-alb-sg"
  description = "ALB listener ports for ${var.name}"
  vpc_id      = data.aws_subnet.first.vpc_id

  dynamic "ingress" {
    for_each = var.listeners
    content {
      description = "Listener ${ingress.value.protocol}:${ingress.value.port}"
      from_port   = ingress.value.port
      to_port     = ingress.value.port
      protocol    = "tcp"
      cidr_blocks = local.ingress_cidr
    }
  }

  egress {
    description = "All outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = local.base_tags
}

resource "aws_lb" "this" {
  name               = var.name
  load_balancer_type = "application"
  internal           = var.internal
  subnets            = var.subnet_ids
  security_groups    = local.sg_ids

  tags = local.base_tags
}

# Target group for instance/ip targets: needs port/protocol/vpc_id + health check.
resource "aws_lb_target_group" "ip" {
  count       = local.is_lambda ? 0 : 1
  name        = "${var.name}-tg"
  target_type = var.target_type
  port        = 80
  protocol    = "HTTP"
  vpc_id      = data.aws_subnet.first.vpc_id

  health_check {
    path = var.health_check_path
  }

  tags = local.base_tags
}

# Target group for a Lambda target: no port/protocol/vpc_id.
resource "aws_lb_target_group" "lambda" {
  count       = local.is_lambda ? 1 : 0
  name        = "${var.name}-tg"
  target_type = "lambda"

  tags = local.base_tags
}

resource "aws_lb_listener" "this" {
  for_each = { for l in var.listeners : tostring(l.port) => l }

  load_balancer_arn = aws_lb.this.arn
  port              = each.value.port
  protocol          = each.value.protocol

  # HTTPS listeners need a policy + certificate; HTTP listeners must not set them.
  ssl_policy      = each.value.protocol == "HTTPS" ? "ELBSecurityPolicy-2016-08" : null
  certificate_arn = each.value.protocol == "HTTPS" ? each.value.certificate_arn : null

  default_action {
    type             = "forward"
    target_group_arn = local.target_group_arn
  }
}

# instance/ip targets register directly.
resource "aws_lb_target_group_attachment" "this" {
  for_each = { for t in local.attach_targets : t => t }

  target_group_arn = aws_lb_target_group.ip[0].arn
  target_id        = each.value
}

# A Lambda target needs the ELB invoke permission before it can be attached.
resource "aws_lambda_permission" "lb" {
  count = local.is_lambda && length(var.targets) > 0 ? 1 : 0

  statement_id  = "AllowExecutionFromALB"
  action        = "lambda:InvokeFunction"
  function_name = var.targets[0]
  principal     = "elasticloadbalancing.amazonaws.com"
  source_arn    = aws_lb_target_group.lambda[0].arn
}

resource "aws_lb_target_group_attachment" "lambda" {
  count = local.is_lambda && length(var.targets) > 0 ? 1 : 0

  target_group_arn = aws_lb_target_group.lambda[0].arn
  target_id        = var.targets[0]

  depends_on = [aws_lambda_permission.lb]
}
