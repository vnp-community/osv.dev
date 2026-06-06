variable "namespace"    { type = string; default = "osv-platform" }
variable "storage_size" { type = string; default = "20Gi" }

resource "helm_release" "redis" {
  name       = "redis"
  repository = "https://charts.bitnami.com/bitnami"
  chart      = "redis"
  namespace  = var.namespace
  version    = "19.6.4"

  values = [templatefile("${path.module}/values.yaml", {
    storage_size = var.storage_size
  })]
}

output "redis_endpoint" {
  value = "redis://redis-master.${var.namespace}.svc.cluster.local:6379"
}
