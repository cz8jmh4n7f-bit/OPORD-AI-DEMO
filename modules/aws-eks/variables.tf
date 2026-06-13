variable "region" {
  type        = string
  description = "AWS region, e.g. eu-central-1."
}

variable "cluster_name" {
  type        = string
  description = "EKS cluster name."
}

variable "kubernetes_version" {
  type        = string
  description = "Kubernetes minor version for the control plane, e.g. \"1.31\"."
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for the control plane ENIs and the node group (>= 2, in different AZs)."
}

variable "node_instance_type" {
  type        = string
  description = "EC2 instance type for the managed node group."
  default     = "t3.medium"
}

variable "node_desired_size" {
  type        = number
  description = "Desired number of worker nodes."
  default     = 2
}

variable "node_min_size" {
  type        = number
  description = "Minimum number of worker nodes."
  default     = 1
}

variable "node_max_size" {
  type        = number
  description = "Maximum number of worker nodes."
  default     = 3
}

variable "environment" {
  type        = string
  description = "Environment tag."
  default     = "dev"
}
