resource "aws_security_group" "lb" {
  name   = "${local.app_name}-alb-sg"
  vpc_id = aws_vpc.app.id

  ingress {
    from_port        = 443
    to_port          = 443
    protocol         = "tcp"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  egress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }
}

resource "aws_alb" "app" {
  name            = "${local.app_name}-alb"
  internal        = false
  security_groups = [aws_security_group.lb.id]
  subnets         = aws_subnet.public[*].id

  tags = {
    Name = "${local.app_name}-alb"
  }
}

resource "aws_lb_listener" "app" {
  load_balancer_arn = aws_alb.app.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-2016-08"
  certificate_arn   = data.aws_acm_certificate.cert.arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app.arn
  }
}

resource "aws_lb_target_group" "app" {
  name        = "${local.app_name}-tg"
  port        = 80
  protocol    = "HTTP"
  vpc_id      = aws_vpc.app.id
  target_type = "instance"


  health_check {
    port     = "traffic-port"
    protocol = "HTTP"
    path     = "/healthz"
  }
}

resource "aws_iam_role" "ssm" {
  name = "${local.app_name}-ssm"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.ssm.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "ssm" {
  name = "${local.app_name}-ssm"
  role = aws_iam_role.ssm.name
}

resource "aws_launch_template" "app" {
  name          = "${local.app_name}-lt"
  image_id      = data.aws_ami.app.id
  instance_type = var.instance_type

  iam_instance_profile {
    name = aws_iam_instance_profile.ssm.name
  }

  user_data = base64encode(templatefile("${path.module}/templates/user_data.sh.tpl", {
    addr            = var.app_addr,
    db_dsn          = "host=${aws_db_instance.app.address} port=${aws_db_instance.app.port} user=${var.db_user} password=${var.db_password} dbname=${var.db_name} sslmode=require",
    allowed_origins = "https://${local.domain_name}",
    b64_signing_key = base64encode(var.signing_key)
  }))

  network_interfaces {
    associate_public_ip_address = true
    security_groups             = [aws_security_group.app.id]
  }
}

resource "aws_autoscaling_group" "app" {
  launch_template {
    id      = aws_launch_template.app.id
    version = "$Latest"
  }
  min_size            = 1
  max_size            = 1
  desired_capacity    = 1
  vpc_zone_identifier = aws_subnet.private[*].id
  target_group_arns   = [aws_lb_target_group.app.arn]

  instance_maintenance_policy {
    max_healthy_percentage = 100
    min_healthy_percentage = 90
  }

  tag {
    key                 = "Name"
    value               = "${local.app_name}-instance"
    propagate_at_launch = true
  }
}

resource "aws_security_group" "app" {
  name   = "${local.app_name}-sg"
  vpc_id = aws_vpc.app.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = aws_subnet.public[*].cidr_block
  }

  egress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  tags = {
    Name = "${local.app_name}-sg"
  }
}
