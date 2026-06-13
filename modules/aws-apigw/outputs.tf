output "endpoint" {
  description = "The default invoke URL of the HTTP API."
  value       = aws_apigatewayv2_api.this.api_endpoint
}

output "api_id" {
  description = "The API Gateway HTTP API id."
  value       = aws_apigatewayv2_api.this.id
}

output "arn" {
  description = "The ARN of the API Gateway HTTP API."
  value       = aws_apigatewayv2_api.this.arn
}
