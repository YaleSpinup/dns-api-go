output "api_gateway_id" {
  description = "ID of the shared HTTP API Gateway"
  value       = module.shared.api_gateway_id
}

output "api_gateway_execution_arn" {
  description = "Execution ARN of the shared HTTP API Gateway"
  value       = module.shared.api_gateway_execution_arn
}

output "api_gateway_endpoint" {
  description = "Default endpoint URL of the HTTP API Gateway"
  value       = module.shared.api_gateway_endpoint
}

output "lambda_execution_role_arn" {
  description = "ARN of the shared Lambda execution role"
  value       = module.shared.lambda_execution_role_arn
}

output "lambda_execution_role_name" {
  description = "Name of the shared Lambda execution role"
  value       = module.shared.lambda_execution_role_name
}

output "vpc_subnet_ids" {
  description = "Subnet IDs for Lambda VPC configuration"
  value       = module.shared.vpc_subnet_ids
}

output "vpc_security_group_ids" {
  description = "Security group IDs for Lambda VPC configuration"
  value       = module.shared.vpc_security_group_ids
}

