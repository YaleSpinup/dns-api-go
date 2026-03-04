# Shared Lambda Infrastructure Module

Provisions the shared AWS infrastructure for Lambda-based microservices behind a single HTTP API Gateway.

## Resources Created

| Resource | Description |
|----------|-------------|
| HTTP API Gateway v2 | Shared gateway for all Lambda services ($1/million requests) |
| API Gateway Stage | Auto-deploying default stage with access logging |
| CloudWatch Log Group | Access logs for API Gateway |
| Security Group | Lambda SG allowing outbound HTTPS and DNS |
| IAM Role | Lambda execution role with VPC, CloudWatch, and Secrets Manager access |
| NAT Gateway (optional) | For Lambda internet access from private subnets |
| Custom Domain (optional) | API Gateway custom domain with TLS |

## Usage

```hcl
module "shared" {
  source = "./modules/shared"

  project_name       = "shared-lambda-api"
  environment        = "dev"
  vpc_id             = "vpc-xxx"
  private_subnet_ids = ["subnet-aaa", "subnet-bbb"]

  # Optional
  create_nat_gateway = false
  custom_domain_name = ""
  certificate_arn    = ""
}
```

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|----------|
| project_name | Project name for resource naming | string | "shared-lambda-api" | no |
| environment | Deployment environment | string | — | yes |
| vpc_id | Existing VPC ID | string | — | yes |
| private_subnet_ids | Private subnet IDs for Lambda | list(string) | — | yes |
| public_subnet_ids | Public subnet IDs for NAT Gateway | list(string) | [] | no |
| create_nat_gateway | Create a NAT Gateway | bool | false | no |
| api_gateway_name | API Gateway name | string | "shared-lambda-api" | no |
| api_gateway_stage_name | Stage name | string | "$default" | no |
| custom_domain_name | Custom domain (optional) | string | "" | no |
| certificate_arn | ACM cert ARN for custom domain | string | "" | no |
| lambda_security_group_egress_cidrs | Egress CIDRs for Lambda SG | list(string) | ["0.0.0.0/0"] | no |
| tags | Additional resource tags | map(string) | {} | no |

## Outputs

| Name | Description |
|------|-------------|
| api_gateway_id | HTTP API Gateway ID |
| api_gateway_execution_arn | API Gateway execution ARN (for Lambda permissions) |
| api_gateway_endpoint | Default API Gateway endpoint URL |
| lambda_execution_role_arn | Lambda execution role ARN |
| lambda_execution_role_name | Lambda execution role name |
| vpc_subnet_ids | Subnet IDs for Lambda VPC config |
| vpc_security_group_ids | Security group IDs for Lambda VPC config |

## Per-Service Integration

Each service module references these outputs:

```hcl
# In a per-service module (e.g., dns-api-go):
data "terraform_remote_state" "shared" {
  backend = "s3"
  config = {
    bucket = "your-terraform-state-bucket"
    key    = "shared-lambda-api/terraform.tfstate"
    region = "us-east-1"
  }
}

resource "aws_lambda_function" "service" {
  # ...
  role = data.terraform_remote_state.shared.outputs.lambda_execution_role_arn

  vpc_config {
    subnet_ids         = data.terraform_remote_state.shared.outputs.vpc_subnet_ids
    security_group_ids = data.terraform_remote_state.shared.outputs.vpc_security_group_ids
  }
}

resource "aws_apigatewayv2_integration" "service" {
  api_id             = data.terraform_remote_state.shared.outputs.api_gateway_id
  integration_type   = "AWS_PROXY"
  integration_uri    = aws_lambda_function.service.invoke_arn
  integration_method = "POST"
}
```

