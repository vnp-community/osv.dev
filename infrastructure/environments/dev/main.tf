variable "project_id" { type = string }
variable "region"     { type = string; default = "us-central1" }

terraform {
  required_providers {
    google = { source = "hashicorp/google"; version = "~> 5.0" }
    helm   = { source = "hashicorp/helm";   version = "~> 2.0" }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

module "gke" {
  source       = "../../modules/gke"
  project_id   = var.project_id
  region       = var.region
  cluster_name = "osv-dev"
}

module "nats" {
  source     = "../../modules/nats"
  namespace  = "osv-platform"
  replicas   = 3
  storage_size = "50Gi"
}

module "redis" {
  source       = "../../modules/redis"
  namespace    = "osv-platform"
  storage_size = "20Gi"
}

module "opensearch" {
  source          = "../../modules/opensearch"
  namespace       = "osv-platform"
  master_replicas = 3
  data_replicas   = 3
}

module "gcs" {
  source     = "../../modules/gcs"
  project_id = var.project_id
  region     = var.region
}

module "iam" {
  source       = "../../modules/iam"
  project_id   = var.project_id
  cluster_name = module.gke.cluster_name
  region       = var.region
}
