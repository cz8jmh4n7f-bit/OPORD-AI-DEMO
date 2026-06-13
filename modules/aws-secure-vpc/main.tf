data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  base_tags = merge(var.tags, { ManagedBy = "opord" })
  azs       = slice(data.aws_availability_zones.available.names, 0, var.az_count)
}

resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = merge(local.base_tags, { Name = "${var.name}-vpc" })
}

# Carve the /22 into /24s, one per AZ (cidrsubnet adds 2 bits to up to four /24s).
resource "aws_subnet" "this" {
  count             = var.az_count
  vpc_id            = aws_vpc.this.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 2, count.index)
  availability_zone = local.azs[count.index]

  # Zero Trust: no auto-assign public IPs. Public egress is opt-in per workload.
  map_public_ip_on_launch = false
  tags                    = merge(local.base_tags, { Name = "${var.name}-subnet-${count.index}" })
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
  tags   = merge(local.base_tags, { Name = "${var.name}-igw" })
}

# Workload route table. Default (no NAT): 0.0.0.0/0 to IGW (but instances have no
# public IP, so this is effectively egress-free - the ZTNA default). With NAT: the
# workload subnets route egress through the NAT gateway instead, so private nodes
# (EKS managed node groups) can reach the EKS API / ECR without a public IP.
resource "aws_route_table" "this" {
  vpc_id = aws_vpc.this.id
  dynamic "route" {
    for_each = var.enable_nat ? [1] : []
    content {
      cidr_block     = "0.0.0.0/0"
      nat_gateway_id = aws_nat_gateway.this[0].id
    }
  }
  dynamic "route" {
    for_each = var.enable_nat ? [] : [1]
    content {
      cidr_block = "0.0.0.0/0"
      gateway_id = aws_internet_gateway.this.id
    }
  }
  tags = merge(local.base_tags, { Name = "${var.name}-rt" })
}

resource "aws_route_table_association" "this" {
  count          = var.az_count
  subnet_id      = aws_subnet.this[count.index].id
  route_table_id = aws_route_table.this.id
}

# --- Optional NAT egress (var.enable_nat) for EKS managed node groups ---
# A small public subnet (the spare /24) hosts the NAT gateway; the NAT lets the
# private workload subnets above egress to the internet without public IPs.
resource "aws_subnet" "public" {
  count                   = var.enable_nat ? 1 : 0
  vpc_id                  = aws_vpc.this.id
  cidr_block              = cidrsubnet(var.vpc_cidr, 2, var.az_count) # the 4th /24
  availability_zone       = local.azs[0]
  map_public_ip_on_launch = true
  tags                    = merge(local.base_tags, { Name = "${var.name}-public-nat" })
}

resource "aws_route_table" "public" {
  count  = var.enable_nat ? 1 : 0
  vpc_id = aws_vpc.this.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }
  tags = merge(local.base_tags, { Name = "${var.name}-public-rt" })
}

resource "aws_route_table_association" "public" {
  count          = var.enable_nat ? 1 : 0
  subnet_id      = aws_subnet.public[0].id
  route_table_id = aws_route_table.public[0].id
}

resource "aws_eip" "nat" {
  count  = var.enable_nat ? 1 : 0
  domain = "vpc"
  tags   = merge(local.base_tags, { Name = "${var.name}-nat-eip" })
}

resource "aws_nat_gateway" "this" {
  count         = var.enable_nat ? 1 : 0
  allocation_id = aws_eip.nat[0].id
  subnet_id     = aws_subnet.public[0].id
  tags          = merge(local.base_tags, { Name = "${var.name}-nat" })
  depends_on    = [aws_internet_gateway.this]
}

# Lock down the default SG (deny-by-default): no ingress/egress rules at all.
resource "aws_default_security_group" "this" {
  vpc_id = aws_vpc.this.id
  tags   = merge(local.base_tags, { Name = "${var.name}-default-sg-locked" })
}

# --- VPC Flow Logs to CloudWatch ---
resource "aws_cloudwatch_log_group" "flow" {
  name              = "/opord/vpc/${var.name}/flow-logs"
  retention_in_days = var.flow_log_retention_days
  tags              = local.base_tags
}

data "aws_iam_policy_document" "flow_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["vpc-flow-logs.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "flow" {
  name               = "${var.name}-flow-logs"
  assume_role_policy = data.aws_iam_policy_document.flow_assume.json
  tags               = local.base_tags
}

data "aws_iam_policy_document" "flow_perms" {
  statement {
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:DescribeLogGroups",
      "logs:DescribeLogStreams",
    ]
    resources = ["${aws_cloudwatch_log_group.flow.arn}:*"]
  }
}

resource "aws_iam_role_policy" "flow" {
  name   = "${var.name}-flow-logs"
  role   = aws_iam_role.flow.id
  policy = data.aws_iam_policy_document.flow_perms.json
}

resource "aws_flow_log" "this" {
  vpc_id          = aws_vpc.this.id
  traffic_type    = "ALL"
  iam_role_arn    = aws_iam_role.flow.arn
  log_destination = aws_cloudwatch_log_group.flow.arn
  tags            = merge(local.base_tags, { Name = "${var.name}-flow-log" })
}
