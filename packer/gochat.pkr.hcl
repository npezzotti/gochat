packer {
  required_plugins {
    amazon = {
      version = ">= 1.2.8"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

variable "domain_name" {
  type = string
}

variable "app_addr" {
  type    = string
  default = "localhost:8000"
}

variable "frontend_dir" {
  type    = string
  default = "../frontend/build"
}

variable "bin_path" {
  type    = string
  default = "../bin/gochat"
}

variable "ami_prefix" {
  type    = string
  default = "gochat"
}

variable "instance_type" {
  type    = string
  default = "t3.micro"
}

variable "region" {
  type    = string
  default = "us-east-1"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "amazon-ebs" "ubuntu" {
  ami_name      = "${var.ami_prefix}-${local.timestamp}"
  instance_type = var.instance_type
  region        = var.region
  source_ami_filter {
    filters = {
      name                = "ubuntu/images/*ubuntu-jammy-22.04-amd64-server-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"]
  }
  ssh_username = "ubuntu"
}

build {
  sources = ["sources.amazon-ebs.ubuntu"]

  provisioner "file" {
    content = templatefile("templates/nginx.conf.tpl", {
      domain   = var.domain_name,
      app_addr = var.app_addr
    })

    destination = "/tmp/nginx.conf"
  }

  provisioner "shell" {
    inline = [
      "sudo apt-get update",
      "sudo apt-get install -y nginx",
      "sudo rm /etc/nginx/sites-enabled/default",
      "sudo mv /tmp/nginx.conf /etc/nginx/sites-available/gochat.conf",
      "sudo ln -s /etc/nginx/sites-available/gochat.conf /etc/nginx/sites-enabled/",
      "sudo systemctl enable nginx",
    ]
  }

  provisioner "file" {
    source      = "${path.root}/${var.bin_path}"
    destination = "/tmp/gochat"
  }

  provisioner "file" {
    source      = "${path.root}/${var.frontend_dir}"
    destination = "/tmp/frontend"
  }

  provisioner "file" {
    content     = templatefile("templates/gochat.service.tpl", {})
    destination = "/tmp/gochat.service"
  }

  provisioner "shell" {
    inline = [
      "sudo mkdir -p /opt/gochat",
      "sudo mv /tmp/gochat /opt/gochat/",
      "sudo mv /tmp/frontend /var/www/gochat",
      "sudo useradd -r -s /bin/false gochat",
      "sudo chown -R gochat:gochat /opt/gochat",
      "sudo chmod +x /opt/gochat/gochat",
      "sudo chown -R www-data:www-data /var/www/gochat/",
      "sudo mv /tmp/gochat.service /etc/systemd/system/gochat.service",
    ]
  }
}
