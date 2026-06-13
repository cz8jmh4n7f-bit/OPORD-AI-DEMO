locals {
  use_s3 = var.s3_bucket != ""
}

# Built-in deployment package: a tiny python handler, zipped at apply time.
# Used only when no S3 code is supplied - gives an immediately-invokable function.
data "archive_file" "inline" {
  count       = local.use_s3 ? 0 : 1
  type        = "zip"
  output_path = "${path.module}/.opord-${var.name}-inline.zip"
  source {
    content  = "def handler(event, context):\n    return {\"statusCode\": 200, \"body\": \"hello from OPORD\"}\n"
    filename = "index.py"
  }
}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "${var.name}-opord-lambda"
  assume_role_policy = data.aws_iam_policy_document.assume.json
  tags = {
    ManagedBy   = "opord"
    Environment = var.environment
  }
}

# Basic execution policy: lets the function write CloudWatch logs.
resource "aws_iam_role_policy_attachment" "basic" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "this" {
  function_name = var.name
  role          = aws_iam_role.lambda.arn
  runtime       = var.runtime
  handler       = var.handler
  memory_size   = var.memory_mb
  timeout       = var.timeout_sec

  filename         = local.use_s3 ? null : data.archive_file.inline[0].output_path
  source_code_hash = local.use_s3 ? null : data.archive_file.inline[0].output_base64sha256
  s3_bucket        = local.use_s3 ? var.s3_bucket : null
  s3_key           = local.use_s3 ? var.s3_key : null

  dynamic "environment" {
    # try() guards against a null env_vars (e.g. passed explicitly as null).
    for_each = try(length(var.env_vars), 0) > 0 ? [1] : []
    content {
      variables = var.env_vars
    }
  }

  tags = {
    ManagedBy   = "opord"
    Environment = var.environment
  }

  depends_on = [aws_iam_role_policy_attachment.basic]
}
