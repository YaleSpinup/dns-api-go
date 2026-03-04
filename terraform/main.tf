terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Uncomment and configure for remote state:
  # backend "s3" {
  #   bucket         = "your-terraform-state-bucket"
  #   key            = "shared-lambda-api/terraform.tfstate"
  #   region         = "us-east-1"
  #   dynamodb_table = "terraform-locks"
  #   encrypt        = true
  # }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project   = var.project_name
      ManagedBy = "terraform"
    }
  }
}

module "shared" {
  source = "./modules/shared"

  project_name = var.project_name
  environment  = var.environment
  vpc_id       = var.vpc_id

  private_subnet_ids = var.private_subnet_ids
  public_subnet_ids  = var.public_subnet_ids

  create_nat_gateway = var.create_nat_gateway

  custom_domain_name = var.custom_domain_name
  certificate_arn    = var.certificate_arn

  lambda_security_group_egress_cidrs = var.lambda_security_group_egress_cidrs

  tags = var.tags
}

