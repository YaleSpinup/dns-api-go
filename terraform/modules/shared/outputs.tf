output "api_gateway_id" {
  description = "ID of the HTTP API Gateway"
  value       = aws_apigatewayv2_api.main.id
}

output "api_gateway_execution_arn" {
  description = "Execution ARN of the HTTP API Gateway (used in Lambda permission resources)"
  value       = aws_apigatewayv2_api.main.execution_arn
}

output "api_gateway_endpoint" {
  description = "Default endpoint URL of the HTTP API Gateway"
  value       = aws_apigatewayv2_api.main.api_endpoint
}

output "lambda_execution_role_arn" {
  description = "ARN of the IAM execution role for Lambda functions"
  value       = aws_iam_role.lambda_execution.arn
}

output "lambda_execution_role_name" {
  description = "Name of the IAM execution role for Lambda functions"
  value       = aws_iam_role.lambda_execution.name
}

output "vpc_subnet_ids" {
  description = "Subnet IDs for Lambda VPC configuration"
  value       = var.private_subnet_ids
}

output "vpc_security_group_ids" {
  description = "Security group IDs for Lambda VPC configuration"
  value       = [aws_security_group.lambda.id]
}

