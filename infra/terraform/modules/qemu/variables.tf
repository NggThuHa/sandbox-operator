# ==============================================================================
# BIẾN CỦA MODULE QEMU (được root module truyền xuống)
# ==============================================================================

variable "cluster_name" {
  type        = string
  description = "Tiền tố đặt tên tài nguyên."
}

variable "nodes" {
  type = map(object({
    role      = string # "master" | "worker"
    index     = number # thứ tự trong nhóm, dùng để sinh IP và MAC
    vcpu      = number
    memory_mb = number
  }))
  description = "Khung danh sách node do root module tính trước (local.node_plan)."
}

variable "disk_size_gb" {
  type        = number
  description = "Dung lượng đĩa mỗi node (GB)."
}

variable "ssh_user" {
  type        = string
  description = "User được cloud-init tạo trong VM."
}

variable "ssh_public_key" {
  type        = string
  description = "SSH public key nạp vào VM. Rỗng = tự sinh cặp khoá mới."
}

variable "ssh_private_key_path" {
  type        = string
  description = "Đường dẫn khoá riêng do người dùng cấp (rỗng nếu tự sinh)."
}

variable "key_output_dir" {
  type        = string
  description = "Thư mục lưu khoá riêng khi Terraform tự sinh (thường là path.root)."
}

variable "storage_pool" {
  type        = string
  description = "Storage pool của libvirt."
}

variable "os_image_source" {
  type        = string
  description = "URL hoặc đường dẫn cloud image."
}

variable "create_network" {
  type        = bool
  description = "Tạo network NAT mới hay dùng network sẵn có."
}

variable "network_name" {
  type        = string
  description = "Tên network sẽ tạo (rỗng = <cluster_name>-net)."
}

variable "network_cidr" {
  type        = string
  description = "Dải mạng NAT của cụm."
}

variable "existing_network_name" {
  type        = string
  description = "Tên network libvirt sẵn có khi create_network = false."
}

variable "dns_forwarders" {
  type        = list(string)
  description = "DNS forwarder cấp cho VM."
}

variable "autostart_vms" {
  type        = bool
  description = "VM tự khởi động cùng libvirtd."
}
