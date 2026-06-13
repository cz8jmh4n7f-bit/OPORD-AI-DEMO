locals {
  # Global / control-plane services that must stay reachable from any region,
  # so the region-lock NotAction list never strands an account.
  region_lock_exempt = [
    "iam:*", "organizations:*", "sts:*", "cloudfront:*", "route53:*",
    "waf:*", "wafv2:*", "shield:*", "support:*", "trustedadvisor:*",
    "globalaccelerator:*", "budgets:*", "ce:*", "cur:*", "health:*",
  ]

  # Preventive guardrails. Kept under the 5120-char SCP limit.
  scp_statements = concat([
    {
      Sid       = "DenyLeaveOrganization"
      Effect    = "Deny"
      Action    = ["organizations:LeaveOrganization"]
      Resource  = "*"
    },
    {
      Sid    = "DenyDisablingSecurityServices"
      Effect = "Deny"
      Action = [
        "cloudtrail:StopLogging",
        "cloudtrail:DeleteTrail",
        "guardduty:DeleteDetector",
        "guardduty:DisassociateFromMasterAccount",
        "config:DeleteConfigurationRecorder",
        "config:StopConfigurationRecorder",
        "config:DeleteDeliveryChannel",
        "securityhub:DisableSecurityHub",
      ]
      Resource = "*"
    },
    {
      Sid       = "DenyAccountAndBillingTamper"
      Effect    = "Deny"
      Action    = ["account:CloseAccount", "account:PutAlternateContact", "account:DeleteAlternateContact"]
      Resource  = "*"
    },
    {
      # Require encryption-by-default for new EBS volumes is enforced at the
      # account level (baseline); here we deny turning it OFF org-wide.
      Sid       = "DenyDisableEbsEncryptionByDefault"
      Effect    = "Deny"
      Action    = ["ec2:DisableEbsEncryptionByDefault"]
      Resource  = "*"
    },
    ], var.enable_region_lock ? [
    {
      Sid       = "DenyOutsideAllowedRegions"
      Effect    = "Deny"
      NotAction = local.region_lock_exempt
      Resource  = "*"
      Condition = {
        StringNotEquals = { "aws:RequestedRegion" = var.allowed_regions }
      }
    }
  ] : [])

  scp_doc = jsonencode({
    Version   = "2012-10-17"
    Statement = local.scp_statements
  })

  # Tag policy schema (distinct from IAM): enforce presence of the required keys.
  tag_policy_doc = jsonencode({
    tags = { for k in var.required_tag_keys : k => {
      tag_key                = { "@@assign" = k }
      enforced_for           = { "@@assign" = ["ec2:instance", "ec2:volume", "rds:db", "s3:bucket"] }
    } }
  })
}

resource "aws_organizations_policy" "scp" {
  name        = "${var.name_prefix}guardrails-scp"
  description = "OPORD preventive guardrails (deny security-service tamper, org leave, region lock)."
  type        = "SERVICE_CONTROL_POLICY"
  content     = local.scp_doc
}

resource "aws_organizations_policy_attachment" "scp" {
  for_each  = toset(var.target_ids)
  policy_id = aws_organizations_policy.scp.id
  target_id = each.value
}

resource "aws_organizations_policy" "tags" {
  count       = var.enable_tag_policy ? 1 : 0
  name        = "${var.name_prefix}required-tags"
  description = "OPORD required-tag enforcement."
  type        = "TAG_POLICY"
  content     = local.tag_policy_doc
}

resource "aws_organizations_policy_attachment" "tags" {
  for_each  = var.enable_tag_policy ? toset(var.target_ids) : toset([])
  policy_id = aws_organizations_policy.tags[0].id
  target_id = each.value
}
