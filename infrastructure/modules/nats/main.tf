variable "namespace"     { type = string; default = "osv-platform" }
variable "replicas"      { type = number; default = 3 }
variable "storage_size"  { type = string; default = "50Gi" }
variable "storage_class" { type = string; default = "premium-rwo" }

resource "helm_release" "nats" {
  name       = "nats"
  repository = "https://nats-io.github.io/k8s/helm/charts/"
  chart      = "nats"
  namespace  = var.namespace
  version    = "1.2.4"

  values = [templatefile("${path.module}/values.yaml", {
    replicas      = var.replicas
    storage_size  = var.storage_size
    storage_class = var.storage_class
  })]
}

output "nats_endpoint" {
  value = "nats://nats.${var.namespace}.svc.cluster.local:4222"
}
