locals {
  resource_prefix = "${var.name_prefix}-${var.environment}"
  api_name        = "${local.resource_prefix}-api"
  web_name        = "${local.resource_prefix}-web"
  db_instance     = "${local.resource_prefix}-pg"
  labels = {
    app         = "aleth"
    environment = var.environment
    managed_by  = "terraform"
  }
}

resource "google_project_service" "required" {
  for_each = toset([
    "artifactregistry.googleapis.com",
    "compute.googleapis.com",
    "run.googleapis.com",
    "secretmanager.googleapis.com",
    "sqladmin.googleapis.com",
  ])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_artifact_registry_repository" "containers" {
  location      = var.region
  repository_id = "${local.resource_prefix}-containers"
  description   = "Aleth container images"
  format        = "DOCKER"
  labels        = local.labels

  depends_on = [google_project_service.required]
}

resource "google_service_account" "api" {
  account_id   = "${local.resource_prefix}-api"
  display_name = "Aleth API ${var.environment}"
  depends_on   = [google_project_service.required]
}

resource "google_service_account" "web" {
  account_id   = "${local.resource_prefix}-web"
  display_name = "Aleth Web ${var.environment}"
  depends_on   = [google_project_service.required]
}

resource "random_password" "db" {
  length  = 32
  special = true
}

resource "google_sql_database_instance" "main" {
  name                = local.db_instance
  database_version    = var.db_version
  region              = var.region
  deletion_protection = var.deletion_protection

  settings {
    tier              = var.db_tier
    availability_type = var.db_availability_type
    disk_size         = var.db_disk_size_gb
    disk_autoresize   = true
    user_labels       = local.labels

    backup_configuration {
      enabled                        = true
      start_time                     = "03:00"
      point_in_time_recovery_enabled = true
    }

    ip_configuration {
      ipv4_enabled = false
    }
  }

  depends_on = [google_project_service.required]
}

resource "google_sql_database" "app" {
  name     = var.db_name
  instance = google_sql_database_instance.main.name
}

resource "google_sql_user" "app" {
  name     = var.db_user
  instance = google_sql_database_instance.main.name
  password = random_password.db.result
}

resource "google_secret_manager_secret" "db_dsn" {
  secret_id = "${local.resource_prefix}-db-dsn"
  labels    = local.labels

  replication {
    auto {}
  }

  depends_on = [google_project_service.required]
}

resource "google_secret_manager_secret_version" "db_dsn" {
  secret = google_secret_manager_secret.db_dsn.id

  secret_data = join(" ", [
    "user=${var.db_user}",
    "password=${random_password.db.result}",
    "dbname=${var.db_name}",
    "host=/cloudsql/${google_sql_database_instance.main.connection_name}",
    "sslmode=disable",
  ])

  depends_on = [
    google_sql_database.app,
    google_sql_user.app,
  ]
}

resource "google_project_iam_member" "api_secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.api.email}"
}

resource "google_project_iam_member" "api_cloudsql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.api.email}"
}

resource "google_cloud_run_v2_service" "api" {
  name     = local.api_name
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"
  labels   = local.labels

  template {
    service_account = google_service_account.api.email

    scaling {
      min_instance_count = var.api_min_instances
      max_instance_count = var.api_max_instances
    }

    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.main.connection_name]
      }
    }

    containers {
      image = var.api_image

      ports {
        container_port = 8080
      }

      env {
        name  = "LEITH_STORE_BACKEND"
        value = "sql"
      }

      env {
        name  = "LEITH_DB_DRIVER"
        value = "pgx"
      }

      env {
        name = "LEITH_DB_DSN"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.db_dsn.secret_id
            version = "latest"
          }
        }
      }

      env {
        name  = "WEBAUTHN_RP_ID"
        value = var.webauthn_rp_id
      }

      env {
        name  = "WEBAUTHN_RP_ORIGINS"
        value = var.webauthn_rp_origins
      }

      resources {
        limits = {
          cpu    = var.api_cpu
          memory = var.api_memory
        }
      }

      volume_mounts {
        name       = "cloudsql"
        mount_path = "/cloudsql"
      }
    }
  }

  depends_on = [
    google_project_iam_member.api_cloudsql_client,
    google_project_iam_member.api_secret_accessor,
    google_secret_manager_secret_version.db_dsn,
  ]
}

resource "google_cloud_run_v2_service" "web" {
  name     = local.web_name
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"
  labels   = local.labels

  template {
    service_account = google_service_account.web.email

    scaling {
      min_instance_count = var.web_min_instances
      max_instance_count = var.web_max_instances
    }

    containers {
      image = var.web_image

      ports {
        container_port = 3000
      }

      env {
        name  = "NEXT_PUBLIC_API_URL"
        value = var.enable_load_balancer && var.domain_name != "" ? "https://${var.domain_name}/api" : google_cloud_run_v2_service.api.uri
      }

      resources {
        limits = {
          cpu    = var.web_cpu
          memory = var.web_memory
        }
      }
    }
  }

  depends_on = [google_cloud_run_v2_service.api]
}

resource "google_cloud_run_v2_service_iam_member" "api_public" {
  count    = var.allow_unauthenticated ? 1 : 0
  name     = google_cloud_run_v2_service.api.name
  location = google_cloud_run_v2_service.api.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_v2_service_iam_member" "web_public" {
  count    = var.allow_unauthenticated ? 1 : 0
  name     = google_cloud_run_v2_service.web.name
  location = google_cloud_run_v2_service.web.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}

