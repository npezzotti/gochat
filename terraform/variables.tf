variable "region" {
  description = "The AWS region to deploy resources in"
  type        = string
  default     = "us-east-1"
}

variable "route53_zone_id" {
  description = "The Route 53 Hosted Zone ID"
  type        = string
}

variable "instance_type" {
  description = "The EC2 instance type"
  type        = string
  default     = "t3.micro"
}

variable "acm_certificate_domain" {
  description = "The domain name for the ACM certificate"
  type        = string
}

/* RDS Configuration */

variable "db_user" {
  description = "The database username"
  type        = string
  default     = "gochat"
}

variable "db_password" {
  description = "The database password"
  type        = string
}

variable "db_name" {
  description = "The database name"
  type        = string
  default     = "gochat"
}

variable "db_port" {
  description = "The port for the database"
  type        = number
  default     = 5432
}

variable "db_instance_class" {
  description = "The database instance class"
  type        = string
  default     = "db.t3.micro"
}

variable "db_allocated_storage" {
  description = "The allocated storage for the database in GB"
  type        = number
  default     = 20
}

/* Application Configuration */

variable "signing_key" {
  description = "The base64 encoded signing key for JWT"
  type        = string
}

variable "app_addr" {
  description = "The address the app will run on"
  type        = string
  default     = "localhost:8000"
}

