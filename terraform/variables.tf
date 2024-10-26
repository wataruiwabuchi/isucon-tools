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
