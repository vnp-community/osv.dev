variable "project_id"   { type = string }
variable "cluster_name" { type = string }
variable "region"       { type = string }

locals {
  services = [
    "api-gateway", "vulnerability-query", "ingestion",
    "source-sync", "impact-analysis", "version-index",
    "search", "web-bff", "ai-enrichment",
    "alias-relations", "notification",
  ]
}

# Service accounts for each microservice
resource "google_service_account" "services" {
  for_each   = toset(local.services)
  account_id = "osv-${each.key}"
  project    = var.project_id
  display_name = "OSV ${each.key} Service Account"
}

# Workload Identity binding: K8s SA → GCP SA
resource "google_service_account_iam_binding" "workload_identity" {
  for_each           = toset(local.services)
  service_account_id = google_service_account.services[each.key].name
  role               = "roles/iam.workloadIdentityUser"
  members = [
    "serviceAccount:${var.project_id}.svc.id.goog[osv-${split("-", each.key)[0]}/${each.key}-sa]"
  ]
}

# Firestore access for data services
resource "google_project_iam_member" "firestore_user" {
  for_each = toset(["ingestion", "vulnerability-query", "alias-relations", "notification", "ai-enrichment"])
  project  = var.project_id
  role     = "roles/datastore.user"
  member   = "serviceAccount:${google_service_account.services[each.key].email}"
}

# GCS access
resource "google_project_iam_member" "gcs_access" {
  for_each = toset(["ingestion", "source-sync"])
  project  = var.project_id
  role     = "roles/storage.objectAdmin"
  member   = "serviceAccount:${google_service_account.services[each.key].email}"
}

# Vertex AI access for AI enrichment
resource "google_project_iam_member" "vertex_ai_user" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.services["ai-enrichment"].email}"
}

output "service_account_emails" {
  value = { for k, v in google_service_account.services : k => v.email }
}
