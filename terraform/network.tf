
resource "aws_vpc" "app" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "${local.app_name}-vpc"
  }
}

resource "aws_internet_gateway" "app" {
  vpc_id = aws_vpc.app.id

  tags = {
    Name = "${local.app_name}-ig"
  }
}

resource "aws_subnet" "public" {
  count = 2

  vpc_id                  = aws_vpc.app.id
  cidr_block              = "10.0.${count.index + 1}.0/24"
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name = "${local.app_name}-public-subnet-${count.index}"
    Type = "public"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.app.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.app.id
  }

  route {
    ipv6_cidr_block = "::/0"
    gateway_id = aws_internet_gateway.app.id
  }

  tags = {
    Name = "${local.app_name}-public-rt"
  }
}

resource "aws_route_table_association" "public" {
  count = 2

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_nat_gateway" "app" {
  count = 2

  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id

  depends_on = [aws_internet_gateway.app]
}

resource "aws_eip" "nat" {
  count = 2

  domain = "vpc"

  depends_on = [aws_internet_gateway.app]
}

resource "aws_subnet" "private" {
  count = 2

  vpc_id            = aws_vpc.app.id
  cidr_block        = "10.0.${count.index + 10}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name = "${local.app_name}-private-subnet-${count.index}"
    Type = "private"
  }
}

resource "aws_route_table" "private" {
  count = 2

  vpc_id = aws_vpc.app.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.app[count.index].id
  }

  tags = {
    Name = "${local.app_name}-private-rt-${count.index + 1}"
  }
}

resource "aws_route_table_association" "private" {
  count = 2

  subnet_id       = aws_subnet.private[count.index].id
  route_table_id  = aws_route_table.private[count.index].id
}

resource "aws_route53_record" "app" {
  zone_id = data.aws_route53_zone.zone.id
  name    = "${local.domain_name}"
  type    = "A"

  alias {
    name                   = aws_alb.app.dns_name
    zone_id                = aws_alb.app.zone_id
    evaluate_target_health = true
  }
}
