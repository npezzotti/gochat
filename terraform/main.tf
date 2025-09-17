provider "aws" {
  region = var.region

  default_tags {
    tags = {
      App = local.app_name
    }
  }
}

locals {
  app_name    = "gochat"
  domain_name = "${local.app_name}.${data.aws_route53_zone.zone.name}"
}
