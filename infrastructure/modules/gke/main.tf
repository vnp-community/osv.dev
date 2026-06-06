variable "project_id" { type = string }
variable "region"     { type = string; default = "us-central1" }
variable "cluster_name" { type = string; default = "osv-cluster" }

resource "google_container_cluster" "osv" {
  name     = var.cluster_name
  location = var.region
  project  = var.project_id

  # GKE Autopilot — fully managed
  enable_autopilot = true

  ip_allocation_policy {}

  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  release_channel {
    channel = "REGULAR"
  }

  addons_config {
    http_load_balancing { disabled = false }
    horizontal_pod_autoscaling { disabled = false }
  }

  maintenance_policy {
    recurring_window {
      start_time = "2023-01-01T02:00:00Z"
      end_time   = "2023-01-01T06:00:00Z"
      recurrence = "FREQ=WEEKLY;BYDAY=SA,SU"
    }
  }
}

output "cluster_name"     { value = google_container_cluster.osv.name }
output "cluster_endpoint" { value = google_container_cluster.osv.endpoint }
