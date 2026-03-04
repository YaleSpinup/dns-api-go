variable "function_name" {
  description = "Name of the Lambda function"
  type        = string
  default     = "dns-api-go"
}

variable "runtime" {
  description = "Lambda runtime"
  type        = string
  default     = "provided.al2023"
}

variable "handler" {
  description = "Lambda handler"
  type        = string
  default     = "bootstrap"
}

variable "memory_size" {
  description = "Lambda memory size in MB"
  type        = number
  default     = 256
}

variable "timeout" {
  description = "Lambda timeout in seconds"
  type        = number
  default     = 30
}

variable "environment" {
  description = "Deployment environment (dev, test, prod)"
  type        = string
}

variable "lambda_zip_path" {
  description = "Path to the Lambda deployment zip file"
  type        = string
  default     = "../../../function.zip"
}

variable "api_config" {
  description = "Base64-encoded API configuration JSON"
  type        = string
  sensitive   = true
}

variable "environment_variables" {
  description = "Additional environment variables for the Lambda function"
  type        = map(string)
  default     = {}
}

# Shared infrastructure references
variable "api_gateway_id" {
  description = "ID of the shared HTTP API Gateway"
  type        = string
}

variable "api_gateway_execution_arn" {
  description = "Execution ARN of the shared HTTP API Gateway"
  type        = string
}

variable "lambda_execution_role_arn" {
  description = "ARN of the shared Lambda execution IAM role"
  type        = string
}

variable "vpc_subnet_ids" {
  description = "Subnet IDs for Lambda VPC configuration"
  type        = list(string)
  default     = []
}

variable "vpc_security_group_ids" {
  description = "Security group IDs for Lambda VPC configuration"
  type        = list(string)
  default     = []
}

variable "log_retention_days" {
  description = "CloudWatch log retention in days"
  type        = number
  default     = 30
}

variable "tags" {
  description = "Additional tags for resources"
  type        = map(string)
  default     = {}
}

