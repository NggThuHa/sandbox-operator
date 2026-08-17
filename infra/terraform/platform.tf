# ==============================================================================
# FILE LỰA CHỌN NỀN TẢNG (PLATFORM SELECTION)
# ==============================================================================
# Đây là nơi duy nhất cần sửa để chuyển đổi hạ tầng bên dưới hoặc bản phân phối
# Kubernetes bên trên:
#
#   cloud_provider = "qemu"   # hạ tầng: VM cục bộ qua libvirt/QEMU-KVM
#   k8s_distro     = "k3s"    # bản phân phối: k3s (nhẹ) hoặc kubeadm
#
# LỘ TRÌNH MỞ RỘNG SANG AWS / AZURE / GCP:
#   1. Tạo modules/<provider>/ với cùng "contract" output như modules/qemu:
#        - nodes                 : map(name => { ip, role, ssh_user, id })
#        - ssh_private_key_path  : đường dẫn khoá riêng cho Ansible
#        - ssh_private_key_openssh (sensitive) : nội dung khoá, dùng cho remote-exec
#   2. Bỏ comment khối module tương ứng trong main.tf.
#   3. Thêm tên provider vào local.implemented_providers bên dưới.
#   Toàn bộ ansible.tf / outputs.tf KHÔNG cần sửa vì chúng chỉ đọc local.nodes.
# ==============================================================================

variable "cloud_provider" {
  type        = string
  default     = "qemu"
  description = <<-EOT
    Nền tảng hạ tầng dùng để tạo máy ảo:
      - "qemu"  : libvirt/QEMU-KVM trên máy cục bộ (đã hỗ trợ)
      - "aws"   : EC2 (chưa implement module)
      - "azure" : Azure Virtual Machines (chưa implement module)
      - "gcp"   : Google Compute Engine (chưa implement module)
  EOT

  validation {
    condition     = contains(["qemu", "aws", "azure", "gcp"], var.cloud_provider)
    error_message = "cloud_provider phải là một trong: qemu, aws, azure, gcp."
  }
}

variable "k8s_distro" {
  type        = string
  default     = "k3s"
  description = <<-EOT
    Bản phân phối Kubernetes, được truyền cho playbook cluster.yml qua biến
    `kubernetes_distro`:
      - "k3s"     : Rancher k3s (nhẹ, phù hợp lab máy ảo đơn lẻ)
      - "kubeadm" : kubeadm + containerd + Calico
  EOT

  validation {
    condition     = contains(["k3s", "kubeadm"], var.k8s_distro)
    error_message = "k8s_distro phải là k3s hoặc kubeadm."
  }
}

locals {
  # Danh sách provider đã có module hoàn chỉnh. Thêm tên vào đây khi implement xong.
  implemented_providers = ["qemu"]
}

# Chặn sớm với thông báo rõ ràng thay vì để Terraform báo lỗi mơ hồ ở tầng resource.
resource "null_resource" "platform_guard" {
  triggers = {
    cloud_provider = var.cloud_provider
    k8s_distro     = var.k8s_distro
  }

  lifecycle {
    precondition {
      condition = contains(local.implemented_providers, var.cloud_provider)
      error_message = join(" ", [
        "cloud_provider = \"${var.cloud_provider}\" chưa có module hạ tầng.",
        "Hiện đã hỗ trợ: ${join(", ", local.implemented_providers)}.",
        "Xem hướng dẫn mở rộng ở đầu file platform.tf."
      ])
    }
  }
}
