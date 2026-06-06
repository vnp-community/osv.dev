variable "project_id" { type = string }
variable "region"     { type = string; default = "us-central1" }

# OSV Vulnerability JSON blobs
resource "google_storage_bucket" "osv_vulnz" {
  name          = "${var.project_id}-osv-vulnz"
  location      = "US"
  storage_class = "STANDARD"
  project       = var.project_id
  force_destroy = false

  versioning { enabled = true }
  lifecycle_rule {
    condition { age = 365 }
    action    { type = "SetStorageClass"; storage_class = "NEARLINE" }
  }
}

resource "google_storage_bucket" "osv_repo_cache" {
  name    = "${var.project_id}-osv-repo-cache"
  location = "US"
  project  = var.project_id
  lifecycle_rule {
    condition { age = 7 }
    action    { type = "Delete" }
  }
}

resource "google_storage_bucket" "osv_backups" {
  name    = "${var.project_id}-osv-backups"
  location = "US"
  project  = var.project_id
  versioning { enabled = true }
}

resource "google_storage_bucket" "osv_public_import_logs" {
  name    = "${var.project_id}-osv-public-import-logs"
  location = "US"
  project  = var.project_id
}

output "vulnz_bucket"            { value = google_storage_bucket.osv_vulnz.name }
output "repo_cache_bucket"       { value = google_storage_bucket.osv_repo_cache.name }
output "backups_bucket"          { value = google_storage_bucket.osv_backups.name }
output "public_import_logs_bucket" { value = google_storage_bucket.osv_public_import_logs.name }
