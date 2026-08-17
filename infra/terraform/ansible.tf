# ==============================================================================
# CẦU NỐI TERRAFORM -> ANSIBLE
# ==============================================================================
# Phần này KHÔNG phụ thuộc cloud_provider: nó chỉ đọc local.nodes (contract chung
# của các module hạ tầng), sinh inventory rồi gọi playbook cluster.yml.
#
# Thứ tự thực thi:
#   1. local_file.inventory        -> ghi inventory/lab-cluster/tf-hosts.ini
#   2. null_resource.wait_cloudinit -> SSH vào từng node, chờ cloud-init xong
#   3. null_resource.ansible        -> ansible-playbook cluster.yml
# ==============================================================================

locals {
  ansible_dir = abspath("${path.module}/../ansible")

  # Đặt inventory ngay trong inventory/lab-cluster/ để Ansible tự nạp group_vars
  # (all/all.yml, all/sysbox.yml, k8s_cluster.yml) nằm cạnh đó. Không ghi đè
  # hosts.ini gốc của người dùng.
  inventory_relative = "inventory/lab-cluster/tf-hosts.ini"
  inventory_path     = "${local.ansible_dir}/${local.inventory_relative}"

  # Extra vars luôn thắng mọi nguồn biến khác của Ansible.
  #
  # real_user/real_home BẮT BUỘC phải truyền: role k3s-master lấy
  # lookup('env','USER')/HOME của MÁY CHẠY ANSIBLE rồi chown trên VM. Nếu không
  # ghi đè, task tạo ~/.kube sẽ fail vì user của máy host không tồn tại trong VM.
  ansible_extra_vars = merge({
    kubernetes_distro     = var.k8s_distro
    k3s_version           = var.k3s_version
    openebs_lvm_pool_size = var.openebs_lvm_pool_size
    real_user             = var.ssh_user
    real_home             = "/home/${var.ssh_user}"
  }, var.ansible_extra_vars)

  ansible_extra_vars_arg = replace(jsonencode(local.ansible_extra_vars), "'", "'\\''")

  ansible_skip_tags_arg = length(var.ansible_skip_tags) > 0 ? "--skip-tags '${join(",", var.ansible_skip_tags)}'" : ""

  # Hash nội dung playbook/role/group_vars để chạy lại Ansible khi Ansible đổi.
  ansible_tracked_files = setunion(
    fileset(local.ansible_dir, "cluster.yml"),
    fileset(local.ansible_dir, "roles/**"),
    fileset(local.ansible_dir, "inventory/lab-cluster/group_vars/**"),
  )

  ansible_content_hash = sha1(join("", [
    for f in sort(tolist(local.ansible_tracked_files)) : filesha1("${local.ansible_dir}/${f}")
  ]))

  kubeconfig_path = "${local.ansible_dir}/${var.k8s_distro == "k3s" ? "k3s.yaml" : "admin.conf"}"

  ansible_command = trimspace(join(" ", [
    "ansible-playbook -i ${local.inventory_relative} cluster.yml",
    "-e '${jsonencode(local.ansible_extra_vars)}'",
    local.ansible_skip_tags_arg,
    var.ansible_extra_args,
  ]))
}

# ------------------------------------------------------------------------------
# 1. SINH INVENTORY
# ------------------------------------------------------------------------------
resource "local_file" "inventory" {
  filename        = local.inventory_path
  file_permission = "0644"

  content = templatefile("${path.module}/templates/hosts.ini.tftpl", {
    cloud_provider       = var.cloud_provider
    k8s_distro           = var.k8s_distro
    masters              = local.masters
    workers              = local.workers
    ssh_user             = var.ssh_user
    ssh_private_key_path = local.ssh_private_key_path
  })
}

# ------------------------------------------------------------------------------
# 2. CHỜ CLOUD-INIT HOÀN TẤT TRÊN TỪNG NODE
# ------------------------------------------------------------------------------
# Không chờ bước này thì Ansible sẽ đụng apt lock (cloud-init đang cài package)
# hoặc SSH vào lúc user chưa được tạo xong.
resource "null_resource" "wait_cloudinit" {
  for_each = local.nodes

  triggers = {
    node_id = each.value.id
  }

  connection {
    type        = "ssh"
    host        = each.value.ip
    user        = each.value.ssh_user
    private_key = local.ssh_private_key_openssh != "" ? local.ssh_private_key_openssh : null
    agent       = local.ssh_private_key_openssh == ""
    timeout     = "10m"
  }

  provisioner "remote-exec" {
    inline = [
      "echo '>>> Cho cloud-init tren ${each.key} (${each.value.ip})...'",
      "cloud-init status --wait || true",
      "cloud-init status --long || true",
      "command -v python3 >/dev/null || { echo 'python3 khong co trong VM'; exit 1; }",
    ]
  }
}

# ------------------------------------------------------------------------------
# 3. CHẠY PLAYBOOK CLUSTER.YML
# ------------------------------------------------------------------------------
resource "null_resource" "ansible" {
  count = var.run_ansible ? 1 : 0

  depends_on = [
    null_resource.wait_cloudinit,
    local_file.inventory,
  ]

  triggers = {
    nodes        = join(",", [for name, node in local.nodes : "${name}=${node.ip}"])
    distro       = var.k8s_distro
    ansible_hash = local.ansible_content_hash
    inventory    = md5(local_file.inventory.content)
    skip_tags    = join(",", var.ansible_skip_tags)
    extra_vars   = jsonencode(local.ansible_extra_vars)
    extra_args   = var.ansible_extra_args
  }

  provisioner "local-exec" {
    working_dir = local.ansible_dir
    interpreter = ["/bin/bash", "-c"]

    environment = {
      ANSIBLE_HOST_KEY_CHECKING = "False"
      ANSIBLE_FORCE_COLOR       = "1"
    }

    command = <<-EOT
      set -euo pipefail
      command -v ansible-playbook >/dev/null || {
        echo "Khong tim thay ansible-playbook. Cai bang: sudo apt install ansible" >&2
        exit 1
      }
      echo ">>> Trien khai ${var.k8s_distro} qua Ansible (${length(local.nodes)} node)"
      ansible-playbook -i '${local.inventory_relative}' cluster.yml \
        -e '${local.ansible_extra_vars_arg}' \
        ${local.ansible_skip_tags_arg} ${var.ansible_extra_args}
    EOT
  }
}

# ------------------------------------------------------------------------------
# 4. DỌN KUBECONFIG KHI DESTROY
# ------------------------------------------------------------------------------
# Inventory là local_file nên Terraform tự xoá. Kubeconfig do Ansible fetch về
# nên phải tự dọn, tránh để lại file trỏ tới cụm đã bị xoá.
resource "null_resource" "kubeconfig_cleanup" {
  triggers = {
    kubeconfig = local.kubeconfig_path
  }

  provisioner "local-exec" {
    when       = destroy
    command    = "rm -f '${self.triggers.kubeconfig}'"
    on_failure = continue
  }
}
