variable "project_name" {
  description = "Project name used for resource naming and tagging"
  type        = string
  default     = "shared-lambda-api"
}

variable "environment" {
  description = "Deployment environment (dev, staging, prod)"
  type        = string
}

# --- VPC Configuration ---

variable "vpc_id" {
  description = "ID of the existing VPC where Lambda functions will run"
  type        = string
}

variable "private_subnet_ids" {
  description = "List of private subnet IDs for Lambda VPC configuration"
  type        = list(string)
}

variable "public_subnet_ids" {
  description = "List of public subnet IDs for NAT Gateway placement (required if create_nat_gateway=true)"
  type        = list(string)
  default     = []
}

# --- NAT Gateway ---

variable "create_nat_gateway" {
  description = "Whether to create a NAT Gateway (one may already exist in the VPC)"
  type        = bool
  default     = false
}

# --- API Gateway ---

variable "api_gateway_name" {
  description = "Name for the HTTP API Gateway"
  type        = string
  default     = "shared-lambda-api"
}

variable "api_gateway_stage_name" {
  description = "Stage name for the API Gateway deployment"
  type        = string
  default     = "$default"
}

variable "custom_domain_name" {
  description = "Custom domain name for the API Gateway (optional)"
  type        = string
  default     = ""
}

variable "certificate_arn" {
  description = "ACM certificate ARN for the custom domain (required if custom_domain_name is set)"
  type        = string
  default     = ""
}

# --- Lambda Configuration ---

variable "lambda_security_group_egress_cidrs" {
  description = "CIDR blocks for Lambda security group egress rules (e.g., Bluecat private network)"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

# --- Tags ---

variable "tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}

