variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "ap-northeast-1"
}

variable "ami_id" {
  description = "AMI ID for ISUCON instance"
  type        = string
}

variable "key_name" {
  description = "SSH key pair name"
  type        = string
}

# インスタンス数の変数定義
variable "instance_count" {
  description = "Number of ISUCON instances to create"
  type        = number
  # NOTE 基本的にはベンチ含めて 4 台
  default     = 1
}

# 追加のIPアドレス用の変数
variable "additional_trusted_ips" {
  description = "追加で許可するIPアドレス"
  type        = list(string)
  default     = []
}

# GitHubユーザー変数の定義
variable "github_users" {
  description = "GitHub usernames for SSH key access"
  type        = list(string)
  default     = ["wataruiwabuchi"]
}
