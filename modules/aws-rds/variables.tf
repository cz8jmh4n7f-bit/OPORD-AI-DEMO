variable "region" {
  type        = string
  description = "AWS region, e.g. eu-central-1."
}

variable "name" {
  type        = string
  description = "DB instance identifier."
}

variable "engine" {
  type        = string
  description = "Database engine: postgres or mysql."
  default     = "postgres"
}

variable "engine_version" {
  type        = string
  description = "Engine version, e.g. \"16\"."
  default     = ""
}

variable "instance_class" {
  type        = string
  description = "RDS instance class, e.g. db.t3.micro."
  default     = "db.t3.micro"
}

variable "storage_gb" {
  type        = number
  description = "Allocated storage (GiB)."
  default     = 20
}

variable "db_name" {
  type        = string
  description = "Initial database name."
  default     = "app"
}

variable "username" {
  type        = string
  description = "Master username (password is managed by RDS in Secrets Manager)."
  default     = "opord"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for the DB subnet group (>= 2 in different AZs)."
  default     = []
}

variable "security_group_ids" {
  type        = list(string)
  description = "Security groups for the DB instance."
  default     = []
}

variable "multi_az" {
  type        = bool
  description = "Deploy across multiple AZs for HA."
  default     = false
}

variable "public_access" {
  type        = bool
  description = "Whether the instance is publicly accessible."
  default     = false
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}
