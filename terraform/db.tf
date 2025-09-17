resource "aws_db_instance" "app" {
  identifier             = "${local.app_name}-db"
  engine                 = "postgres"
  instance_class         = var.db_instance_class
  allocated_storage      = var.db_allocated_storage
  vpc_security_group_ids = [aws_security_group.db.id]
  db_subnet_group_name   = aws_db_subnet_group.db.name
  username               = var.db_user
  password               = var.db_password
  db_name                = var.db_name
  skip_final_snapshot    = true
  port                   = var.db_port

  tags = {
    Name = "${local.app_name}-db"
  }
}

resource "aws_db_subnet_group" "db" {
  name       = "${local.app_name}-db-subnet-group"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "${local.app_name}-db-subnet-group"
  }
}

resource "aws_security_group" "db" {
  name   = "${local.app_name}-db-sg"
  vpc_id = aws_vpc.app.id

  ingress {
    from_port   = var.db_port
    to_port     = var.db_port
    protocol    = "tcp"
    cidr_blocks = aws_subnet.private[*].cidr_block
  }

  egress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  tags = {
    Name = "${local.app_name}-db-sg"
  }
}
