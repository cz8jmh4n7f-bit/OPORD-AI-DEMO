variable "location" {
  type        = string
  description = "Azure region."
  default     = "westeurope"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix; Function App + Storage Account names are bounded (24 chars storage, 60 chars function); module trims."
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}

variable "runtime" {
  type        = string
  description = "Function runtime: python, node, java, dotnet, powershell."
  default     = "python"
}

variable "runtime_version" {
  type        = string
  description = "Runtime version (e.g. 3.12 for python, 20 for node, 17 for java). AKS defaults vary."
  default     = "3.12"
}

variable "env_vars" {
  type        = map(string)
  description = "Application settings (key/value env vars at run time)."
  default     = {}
}

variable "tags" {
  type        = map(string)
  description = "Extra tags merged onto every resource."
  default     = {}
}
