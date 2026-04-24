output "artifact_registry_repository" {
  description = "Artifact Registry repository name."
  value       = google_artifact_registry_repository.containers.name
}

output "artifact_registry_repository_url" {
  description = "Docker repository URL prefix."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.containers.repository_id}"
}

output "api_service_name" {
  description = "Cloud Run API service name."
  value       = google_cloud_run_v2_service.api.name
}

output "api_service_url" {
  description = "Cloud Run API service URL."
  value       = google_cloud_run_v2_service.api.uri
}

output "web_service_name" {
  description = "Cloud Run web service name."
  value       = google_cloud_run_v2_service.web.name
}

output "web_service_url" {
  description = "Cloud Run web service URL."
  value       = google_cloud_run_v2_service.web.uri
}

output "cloud_sql_instance_connection_name" {
  description = "Cloud SQL instance connection name."
  value       = google_sql_database_instance.main.connection_name
}

output "db_dsn_secret_id" {
  description = "Secret Manager secret ID containing LEITH_DB_DSN."
  value       = google_secret_manager_secret.db_dsn.secret_id
}

output "load_balancer_ip" {
  description = "Global HTTPS load balancer IP, if enabled with a domain."
  value       = var.enable_load_balancer && var.domain_name != "" ? google_compute_global_address.main[0].address : null
}

output "custom_domain_url" {
  description = "Custom domain URL, if configured."
  value       = var.enable_load_balancer && var.domain_name != "" ? "https://${var.domain_name}" : null
}

