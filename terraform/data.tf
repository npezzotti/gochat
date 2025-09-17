data "aws_caller_identity" "current" {}
data "aws_availability_zones" "available" {}

data "aws_ami" "app" {
  most_recent = true
  name_regex  = "^gochat-.*"
  owners      = [data.aws_caller_identity.current.account_id]
}

data "aws_route53_zone" "zone" {
  zone_id = var.route53_zone_id
}

data "aws_acm_certificate" "cert" {
  domain   = var.acm_certificate_domain
  statuses = ["ISSUED"]
}
