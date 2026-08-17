# ==============================================================================
# DANH SÁCH NODE (TÍNH TRƯỚC, KHÔNG PHỤ THUỘC HẠ TẦNG)
# ==============================================================================
# node_plan phải được xác định hoàn toàn ở thời điểm plan: các resource dùng
# `for_each` (chờ cloud-init, sinh inventory) lấy key từ đây. IP thì để module
# hạ tầng cung cấp, vì với AWS/Azure IP chỉ biết sau khi apply.
# ==============================================================================

locals {
  master_names = [for i in range(var.master_count) : "${var.cluster_name}-master-${i + 1}"]
  worker_names = [for i in range(var.worker_count) : "${var.cluster_name}-worker-${i + 1}"]

  node_plan = merge(
    {
      for i, name in local.master_names : name => {
        role      = "master"
        index     = i + 1
        vcpu      = var.master_vcpu
        memory_mb = var.master_memory_mb
      }
    },
    {
      for i, name in local.worker_names : name => {
        role      = "worker"
        index     = i + 1
        vcpu      = var.worker_vcpu
        memory_mb = var.worker_memory_mb
      }
    },
  )
}
