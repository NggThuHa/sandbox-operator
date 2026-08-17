# ==============================================================================
# OUTPUT CONTRACT — MỌI MODULE HẠ TẦNG PHẢI TRẢ ĐÚNG BỘ NÀY
# ==============================================================================
# root module (ansible.tf, outputs.tf) chỉ đọc các output dưới đây, nhờ vậy khi
# thêm modules/aws, modules/azure... không phải sửa gì ở tầng trên.
# Lưu ý: key của các map phải trùng key của biến `nodes` được truyền vào.
# ==============================================================================

output "node_ips" {
  description = "Map tên node => IP để SSH/Ansible dùng."
  value       = { for name, node in local.nodes : name => node.ip }
}

output "node_ids" {
  description = "Map tên node => id tài nguyên hạ tầng (tạo dependency ngầm cho provisioner)."
  value       = { for name, node in local.nodes : name => libvirt_domain.node[name].id }
}

output "ssh_private_key_path" {
  description = "Đường dẫn khoá riêng để Ansible/SSH dùng (rỗng nếu dựa vào ssh-agent)."
  value       = local.ssh_private_key_path
}

output "ssh_private_key_openssh" {
  description = "Nội dung khoá riêng cho provisioner remote-exec."
  value       = local.ssh_private_key_openssh
  sensitive   = true
}

output "network" {
  description = "Thông tin mạng của cụm."
  value = {
    name    = local.network_name
    cidr    = var.network_cidr
    gateway = local.network_gateway
    domain  = local.domain_suffix
  }
}
