locals {
  is_lambda      = var.integration_type == "lambda"
  has_target     = var.integration_target != ""
  custom_domain  = var.domain_name != ""
  dns_record     = local.custom_domain && var.hosted_zone_id != ""
  lambda_permits = local.is_lambda && local.has_target
}

# The HTTP API itself.
resource "aws_apigatewayv2_api" "this" {
  name          = var.name
  protocol_type = "HTTP"
  tags          = var.tags
}

# Backend integration: AWS_PROXY for Lambda, HTTP_PROXY for an upstream URL.
resource "aws_apigatewayv2_integration" "this" {
  api_id           = aws_apigatewayv2_api.this.id
  integration_type = local.is_lambda ? "AWS_PROXY" : "HTTP_PROXY"
  integration_uri  = var.integration_target

  # AWS_PROXY (Lambda) uses the 2.0 payload format; HTTP_PROXY uses an HTTP method.
  payload_format_version = local.is_lambda ? "2.0" : null
  integration_method     = local.is_lambda ? null : "ANY"
}

resource "aws_apigatewayv2_route" "this" {
  api_id    = aws_apigatewayv2_api.this.id
  route_key = var.route_key
  target    = "integrations/${aws_apigatewayv2_integration.this.id}"
}

resource "aws_apigatewayv2_stage" "this" {
  api_id      = aws_apigatewayv2_api.this.id
  name        = "$default"
  auto_deploy = true
  tags        = var.tags
}

# Allow API Gateway to invoke the Lambda target.
resource "aws_lambda_permission" "this" {
  count         = local.lambda_permits ? 1 : 0
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = var.integration_target
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.this.execution_arn}/*/*"
}

# Optional custom domain: domain name + API mapping (+ Route53 ALIAS).
resource "aws_apigatewayv2_domain_name" "this" {
  count       = local.custom_domain ? 1 : 0
  domain_name = var.domain_name

  domain_name_configuration {
    certificate_arn = var.certificate_arn
    endpoint_type   = "REGIONAL"
    security_policy = "TLS_1_2"
  }

  tags = var.tags
}

resource "aws_apigatewayv2_api_mapping" "this" {
  count       = local.custom_domain ? 1 : 0
  api_id      = aws_apigatewayv2_api.this.id
  domain_name = aws_apigatewayv2_domain_name.this[0].id
  stage       = aws_apigatewayv2_stage.this.id
}

resource "aws_route53_record" "this" {
  count   = local.dns_record ? 1 : 0
  zone_id = var.hosted_zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = aws_apigatewayv2_domain_name.this[0].domain_name_configuration[0].target_domain_name
    zone_id                = aws_apigatewayv2_domain_name.this[0].domain_name_configuration[0].hosted_zone_id
    evaluate_target_health = false
  }
}
