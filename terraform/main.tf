provider "aws" {
  region = var.aws_region
}

# VPC設定
resource "aws_vpc" "isucon_vpc" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "isucon-vpc"
  }
}

# パブリックサブネット
resource "aws_subnet" "public" {
  vpc_id            = aws_vpc.isucon_vpc.id
  cidr_block        = "10.0.1.0/24"
  availability_zone = "${var.aws_region}a"

  tags = {
    Name = "isucon-public-subnet"
  }
}

# インスタンス数の変数定義
variable "instance_count" {
  description = "Number of ISUCON instances to create"
  type        = number
  # NOTE 基本的にはベンチ含めて 4 台
  default     = 1
}

# ISUCON用EC2インスタンス
data "aws_iam_role" "ssm_role" {
  name = "ec2-ssm-isucon13"
}

resource "aws_iam_instance_profile" "isucon_profile" {
  name = "isucon13-instance-profile"
  role = data.aws_iam_role.ssm_role.name
}

# ISUCON用EC2インスタンス
resource "aws_instance" "isucon" {
  count                = var.instance_count
  ami                  = var.ami_id
  # instance_type = "c5.large"  # 基本的には c5.large
  instance_type        = "t3.small"
  iam_instance_profile = aws_iam_instance_profile.isucon_profile.name

  subnet_id                   = aws_subnet.public.id
  associate_public_ip_address = true
  key_name                   = var.key_name
  vpc_security_group_ids     = [aws_security_group.isucon.id]

  root_block_device {
    volume_size = 20
    volume_type = "gp3"
  }

  user_data = <<-EOF
              #!/usr/bin/env bash
              export HOME=/home/isucon
              mkdir -p $HOME/.ssh
              touch $HOME/.ssh/authorized_keys
              chmod 700 $HOME/.ssh
              chmod 600 $HOME/.ssh/authorized_keys

              # GitHubから公開鍵を取得して追加
              %{for user in var.github_users}
              curl -s https://github.com/${user}.keys >> $HOME/.ssh/authorized_keys
              %{endfor}

              chown -R isucon:isucon $HOME/.ssh
              EOF

  tags = {
    Name = "isucon-instance-${count.index + 1}"
  }
}

# 現在のIPアドレスを取得
data "http" "my_ip" {
  url = "https://api.ipify.org"
}

# 追加のIPアドレス用の変数
variable "additional_trusted_ips" {
  description = "追加で許可するIPアドレス"
  type        = list(string)
  default     = []
}

locals {
  current_ip = chomp(data.http.my_ip.response_body)
  all_trusted_ips = concat(
    ["${local.current_ip}/32"],
    var.additional_trusted_ips
  )
}

# セキュリティグループの設定
resource "aws_security_group" "isucon" {
  name        = "isucon-sg"
  description = "Security group for ISUCON"
  vpc_id      = aws_vpc.isucon_vpc.id

  # NOTE isucon なので広めに空けておく
  ingress {
    from_port   = 0
    to_port     = 65535
    protocol    = "tcp"
    cidr_blocks = local.all_trusted_ips
    description = "Allow all TCP inbound traffic"
  }

  # 全てのアウトバウンドトラフィックを許可
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound traffic"
  }

  tags = {
    Name = "isucon-security-group"
  }
}
