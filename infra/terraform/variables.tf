variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "region" {
  description = "Primary GCP region."
  type        = string
  default     = "asia-east1"
}

variable "environment" {
  description = "Deployment environment name, such as staging or prod."
  type        = string
  default     = "staging"
}

variable "name_prefix" {
  description = "Prefix used for Aleth resources."
  type        = string
  default     = "aleth"
}

variable "api_image" {
  description = "Container image URI for the Go API Cloud Run service."
  type        = string
}

variable "web_image" {
  description = "Container image URI for the Next.js web Cloud Run service."
  type        = string
}

variable "domain_name" {
  description = "Optional custom domain for the HTTPS load balancer. Leave empty to skip LB resources."
  type        = string
  default     = ""
}

variable "enable_load_balancer" {
  description = "Whether to create a global HTTPS load balancer that routes /api/* to the API and everything else to web."
  type        = bool
  default     = false
}

variable "allow_unauthenticated" {
  description = "Whether Cloud Run services allow public unauthenticated invocations."
  type        = bool
  default     = true
}

variable "db_name" {
  description = "Application PostgreSQL database name."
  type        = string
  default     = "aleth"
}

variable "db_user" {
  description = "Application PostgreSQL user."
  type        = string
  default     = "aleth_app"
}

variable "db_tier" {
  description = "Cloud SQL machine tier."
  type        = string
  default     = "db-custom-1-3840"
}

variable "db_version" {
  description = "Cloud SQL PostgreSQL version."
  type        = string
  default     = "POSTGRES_16"
}

variable "db_availability_type" {
  description = "Cloud SQL availability type. Use REGIONAL for production HA."
  type        = string
  default     = "ZONAL"
}

variable "db_disk_size_gb" {
  description = "Cloud SQL disk size in GB."
  type        = number
  default     = 20
}

variable "deletion_protection" {
  description = "Protect destructive deletion of stateful resources."
  type        = bool
  default     = true
}

variable "api_cpu" {
  description = "Cloud Run API CPU limit."
  type        = string
  default     = "1"
}

variable "api_memory" {
  description = "Cloud Run API memory limit."
  type        = string
  default     = "1Gi"
}

variable "web_cpu" {
  description = "Cloud Run web CPU limit."
  type        = string
  default     = "1"
}

variable "web_memory" {
  description = "Cloud Run web memory limit."
  type        = string
  default     = "1Gi"
}

variable "api_min_instances" {
  description = "Minimum API instances."
  type        = number
  default     = 0
}

variable "api_max_instances" {
  description = "Maximum API instances."
  type        = number
  default     = 10
}

variable "web_min_instances" {
  description = "Minimum web instances."
  type        = number
  default     = 0
}

variable "web_max_instances" {
  description = "Maximum web instances."
  type        = number
  default     = 10
}

variable "webauthn_rp_id" {
  description = "Future WebAuthn relying party ID. The current API code must be updated before this is effective."
  type        = string
  default     = ""
}

variable "webauthn_rp_origins" {
  description = "Future comma-separated WebAuthn allowed origins. The current API code must be updated before this is effective."
  type        = string
  default     = ""
}

