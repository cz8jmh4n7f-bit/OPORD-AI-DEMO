output "function_arn" {
  description = "The Lambda function ARN."
  value       = aws_lambda_function.this.arn
}

output "function_name" {
  description = "The Lambda function name."
  value       = aws_lambda_function.this.function_name
}

output "role_arn" {
  description = "The function's execution role ARN."
  value       = aws_iam_role.lambda.arn
}
