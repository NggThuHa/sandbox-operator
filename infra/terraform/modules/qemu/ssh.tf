# ==============================================================================
# KHOÁ SSH
# ==============================================================================
# Nếu người dùng không cấp ssh_public_key, Terraform tự sinh cặp khoá ed25519 và
# ghi khoá riêng ra <thư mục terraform>/.ssh/<cluster_name>_ed25519 (mode 0600).
# Nhờ đó `terraform apply` chạy được trên máy chưa có sẵn khoá nào.
# ==============================================================================

resource "tls_private_key" "ssh" {
  count = var.ssh_public_key == "" ? 1 : 0

  algorithm = "ED25519"
}

resource "local_sensitive_file" "ssh_private_key" {
  count = var.ssh_public_key == "" ? 1 : 0

  content              = tls_private_key.ssh[0].private_key_openssh
  filename             = "${var.key_output_dir}/.ssh/${var.cluster_name}_ed25519"
  file_permission      = "0600"
  directory_permission = "0700"
}

resource "local_file" "ssh_public_key" {
  count = var.ssh_public_key == "" ? 1 : 0

  content              = tls_private_key.ssh[0].public_key_openssh
  filename             = "${var.key_output_dir}/.ssh/${var.cluster_name}_ed25519.pub"
  file_permission      = "0644"
  directory_permission = "0700"
}

locals {
  generated_key = var.ssh_public_key == ""

  ssh_public_key = trimspace(
    local.generated_key ? tls_private_key.ssh[0].public_key_openssh : var.ssh_public_key
  )

  ssh_private_key_path = (
    local.generated_key
    ? abspath(local_sensitive_file.ssh_private_key[0].filename)
    : (var.ssh_private_key_path != "" ? pathexpand(var.ssh_private_key_path) : "")
  )

  # Nội dung khoá riêng dùng cho provisioner remote-exec ở root module. Khi
  # người dùng tự cấp khoá mà không khai báo đường dẫn, để rỗng để dùng ssh-agent.
  ssh_private_key_openssh = (
    local.generated_key
    ? tls_private_key.ssh[0].private_key_openssh
    : (var.ssh_private_key_path != "" ? file(pathexpand(var.ssh_private_key_path)) : "")
  )
}
