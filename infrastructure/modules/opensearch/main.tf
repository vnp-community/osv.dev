variable "namespace"         { type = string; default = "osv-platform" }
variable "master_replicas"   { type = number; default = 3 }
variable "data_replicas"     { type = number; default = 3 }
variable "master_storage"    { type = string; default = "20Gi" }
variable "data_storage"      { type = string; default = "200Gi" }
variable "storage_class"     { type = string; default = "premium-rwo" }

resource "helm_release" "opensearch" {
  name       = "opensearch"
  repository = "https://opensearch-project.github.io/helm-charts/"
  chart      = "opensearch"
  namespace  = var.namespace
  version    = "2.22.0"

  values = [templatefile("${path.module}/values.yaml", {
    master_replicas = var.master_replicas
    data_replicas   = var.data_replicas
    master_storage  = var.master_storage
    data_storage    = var.data_storage
    storage_class   = var.storage_class
  })]
}

output "opensearch_endpoint" {
  value = "http://opensearch-cluster-master.${var.namespace}.svc.cluster.local:9200"
}
