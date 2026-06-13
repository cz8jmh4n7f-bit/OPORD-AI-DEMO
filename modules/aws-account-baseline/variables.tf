variable "region" {
  type        = string
  description = "Home region for account-level resources (Config, budget)."
}

variable "assume_role_arn" {
  type        = string
  description = "Role ARN in the member account to assume."
}

variable "name" {
  type        = string
  description = "Name prefix (e.g. opord-<csa_id>)."
}

variable "monthly_budget_usd" {
  type        = number
  description = "Monthly cost budget; an alert fires at 80% and 100%."
  default     = 500
}

variable "budget_notify_emails" {
  type        = list(string)
  description = "Emails to notify on budget thresholds."
  default     = []
}

variable "enable_config" {
  type        = bool
  description = "Enable the AWS Config recorder + delivery channel."
  default     = true
}

variable "tags" {
  type        = map(string)
  description = "Tags applied to all resources."
  default     = {}
}
