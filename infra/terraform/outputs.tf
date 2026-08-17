# ==============================================================================
# OUTPUTS
# ==============================================================================

output "platform" {
  description = "Nền tảng và bản phân phối đang dùng."
  value = {
    cloud_provider = var.cloud_provider
    k8s_distro     = var.k8s_distro
    cluster_name   = var.cluster_name
  }
}

output "network" {
  description = "Thông tin mạng của cụm (null với provider chưa dùng mạng cục bộ)."
  value       = local.network
}

output "nodes" {
  description = "Danh sách node: IP, vai trò, cấu hình."
  value = {
    for name, node in local.nodes : name => {
      ip        = node.ip
      role      = node.role
      vcpu      = node.vcpu
      memory_mb = node.memory_mb
    }
  }
}

output "master_ip" {
  description = "IP của node control-plane (dùng cho kube-apiserver :6443)."
  value       = local.master_ip
}

output "ssh_private_key_path" {
  description = "Khoá riêng để SSH vào các node."
  value       = local.ssh_private_key_path
}

output "ssh_commands" {
  description = "Lệnh SSH sẵn dùng cho từng node."
  value = {
    for name, node in local.nodes : name => trimspace(join(" ", [
      "ssh",
      local.ssh_private_key_path != "" ? "-i ${local.ssh_private_key_path}" : "",
      "-o StrictHostKeyChecking=no",
      "${node.ssh_user}@${node.ip}",
    ]))
  }
}

output "ansible_inventory_path" {
  description = "Inventory do Terraform sinh ra."
  value       = local.inventory_path
}

output "ansible_command" {
  description = "Lệnh chạy lại Ansible thủ công (cd vào infra/ansible trước)."
  value       = local.ansible_command
}

output "kubeconfig_path" {
  description = "Kubeconfig mà Ansible fetch về máy (đã trỏ tới IP master)."
  value       = local.kubeconfig_path
}

output "next_steps" {
  description = "Việc cần làm sau khi apply xong."
  value       = <<-EOT
    Kiểm tra cụm:
      export KUBECONFIG="${local.kubeconfig_path}"
      kubectl get nodes -o wide
      kubectl get runtimeclass          # mong đợi: sysbox-runc
      kubectl get sc                    # mong đợi: openebs-lvm

    SSH vào master:
      ssh ${local.ssh_private_key_path != "" ? "-i ${local.ssh_private_key_path}" : ""} ${var.ssh_user}@${local.master_ip}

    Chạy lại Ansible mà không tạo lại VM:
      cd ${local.ansible_dir} && ${local.ansible_command}
  EOT
}
