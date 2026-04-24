resource "google_compute_region_network_endpoint_group" "web" {
  count                 = var.enable_load_balancer ? 1 : 0
  name                  = "${local.resource_prefix}-web-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region

  cloud_run {
    service = google_cloud_run_v2_service.web.name
  }
}

resource "google_compute_region_network_endpoint_group" "api" {
  count                 = var.enable_load_balancer ? 1 : 0
  name                  = "${local.resource_prefix}-api-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region

  cloud_run {
    service = google_cloud_run_v2_service.api.name
  }
}

resource "google_compute_backend_service" "web" {
  count                 = var.enable_load_balancer ? 1 : 0
  name                  = "${local.resource_prefix}-web-backend"
  protocol              = "HTTP"
  load_balancing_scheme = "EXTERNAL_MANAGED"

  backend {
    group = google_compute_region_network_endpoint_group.web[0].id
  }
}

resource "google_compute_backend_service" "api" {
  count                 = var.enable_load_balancer ? 1 : 0
  name                  = "${local.resource_prefix}-api-backend"
  protocol              = "HTTP"
  load_balancing_scheme = "EXTERNAL_MANAGED"

  backend {
    group = google_compute_region_network_endpoint_group.api[0].id
  }
}

resource "google_compute_url_map" "main" {
  count           = var.enable_load_balancer ? 1 : 0
  name            = "${local.resource_prefix}-url-map"
  default_service = google_compute_backend_service.web[0].id

  host_rule {
    hosts        = var.domain_name == "" ? ["*"] : [var.domain_name]
    path_matcher = "main"
  }

  path_matcher {
    name            = "main"
    default_service = google_compute_backend_service.web[0].id

    path_rule {
      paths   = ["/api", "/api/*"]
      service = google_compute_backend_service.api[0].id
    }
  }
}

resource "google_compute_managed_ssl_certificate" "main" {
  count = var.enable_load_balancer && var.domain_name != "" ? 1 : 0
  name  = "${local.resource_prefix}-managed-cert"

  managed {
    domains = [var.domain_name]
  }
}

resource "google_compute_target_https_proxy" "main" {
  count   = var.enable_load_balancer && var.domain_name != "" ? 1 : 0
  name    = "${local.resource_prefix}-https-proxy"
  url_map = google_compute_url_map.main[0].id
  ssl_certificates = [
    google_compute_managed_ssl_certificate.main[0].id,
  ]
}

resource "google_compute_global_address" "main" {
  count = var.enable_load_balancer && var.domain_name != "" ? 1 : 0
  name  = "${local.resource_prefix}-lb-ip"
}

resource "google_compute_global_forwarding_rule" "https" {
  count                 = var.enable_load_balancer && var.domain_name != "" ? 1 : 0
  name                  = "${local.resource_prefix}-https"
  target                = google_compute_target_https_proxy.main[0].id
  port_range            = "443"
  ip_address            = google_compute_global_address.main[0].address
  load_balancing_scheme = "EXTERNAL_MANAGED"
}

