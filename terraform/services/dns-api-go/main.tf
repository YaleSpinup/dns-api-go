terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

locals {
  function_name = "${var.function_name}-${var.environment}"

  common_tags = merge(var.tags, {
    Service     = var.function_name
    Environment = var.environment
    ManagedBy   = "terraform"
  })

  environment_variables = merge({
    API_CONFIG = var.api_config
  }, var.environment_variables)
}

# =============================================================================
# CloudWatch Log Group
# =============================================================================

resource "aws_cloudwatch_log_group" "this" {
  name              = "/aws/lambda/${local.function_name}"
  retention_in_days = var.log_retention_days

  tags = local.common_tags
}

# =============================================================================
# Lambda Function
# =============================================================================

resource "aws_lambda_function" "this" {
  function_name = local.function_name
  role          = var.lambda_execution_role_arn
  handler       = var.handler
  runtime       = var.runtime
  memory_size   = var.memory_size
  timeout       = var.timeout

  filename         = var.lambda_zip_path
  source_code_hash = filebase64sha256(var.lambda_zip_path)

  environment {
    variables = local.environment_variables
  }

  dynamic "vpc_config" {
    for_each = length(var.vpc_subnet_ids) > 0 ? [1] : []
    content {
      subnet_ids         = var.vpc_subnet_ids
      security_group_ids = var.vpc_security_group_ids
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.this,
  ]

  tags = local.common_tags
}

# =============================================================================
# API Gateway Integration
# =============================================================================

resource "aws_apigatewayv2_integration" "this" {
  api_id                 = var.api_gateway_id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.this.invoke_arn
  integration_method     = "POST"
  payload_format_version = "2.0"
}

resource "aws_apigatewayv2_route" "this" {
  api_id    = var.api_gateway_id
  route_key = "ANY /v2/dns/{proxy+}"
  target    = "integrations/${aws_apigatewayv2_integration.this.id}"
}

# =============================================================================
# Lambda Permission for API Gateway
# =============================================================================

resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.this.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${var.api_gateway_execution_arn}/*/*"
}

