# ==============================================================================
# DISPATCH HẠ TẦNG THEO cloud_provider
# ==============================================================================
# Mỗi provider là một module con có cùng output contract (xem platform.tf).
# Root module chỉ làm hai việc: chọn module để bật, và gom output về local.nodes.
# ==============================================================================

module "qemu" {
  source = "./modules/qemu"
  count  = var.cloud_provider == "qemu" ? 1 : 0

  cluster_name = var.cluster_name
  nodes        = local.node_plan
  disk_size_gb = var.disk_size_gb

  ssh_user             = var.ssh_user
  ssh_public_key       = var.ssh_public_key
  ssh_private_key_path = var.ssh_private_key_path
  key_output_dir       = path.root

  storage_pool          = var.storage_pool
  os_image_source       = var.os_image_source
  create_network        = var.create_network
  network_name          = var.network_name
  network_cidr          = var.network_cidr
  existing_network_name = var.existing_network_name
  dns_forwarders        = var.dns_forwarders
  autostart_vms         = var.autostart_vms
}

# ------------------------------------------------------------------------------
# CÁC PROVIDER MÂY (chưa implement) — bỏ comment sau khi tạo module tương ứng
# và nhớ thêm tên vào local.implemented_providers trong platform.tf.
# ------------------------------------------------------------------------------
# module "aws" {
#   source = "./modules/aws"
#   count  = var.cloud_provider == "aws" ? 1 : 0
#   nodes  = local.node_plan
#   ...
# }
#
# module "azure" {
#   source = "./modules/azure"
#   count  = var.cloud_provider == "azure" ? 1 : 0
#   ...
# }
#
# module "gcp" {
#   source = "./modules/gcp"
#   count  = var.cloud_provider == "gcp" ? 1 : 0
#   ...
# }

locals {
  # Gom output của module đang bật về một chỗ duy nhất. one() trả null khi module
  # có count = 0 (dùng one() thay try() để giữ được giá trị "known at plan").
  node_ips_by_provider = {
    qemu = one(module.qemu[*].node_ips)
    # aws = one(module.aws[*].node_ips)
  }

  node_ids_by_provider = {
    qemu = one(module.qemu[*].node_ids)
  }

  ssh_key_path_by_provider = {
    qemu = one(module.qemu[*].ssh_private_key_path)
  }

  ssh_key_content_by_provider = {
    qemu = one(module.qemu[*].ssh_private_key_openssh)
  }

  network_by_provider = {
    qemu = one(module.qemu[*].network)
  }

  selected_node_ips    = lookup(local.node_ips_by_provider, var.cloud_provider, null)
  selected_node_ids    = lookup(local.node_ids_by_provider, var.cloud_provider, null)
  selected_key_path    = lookup(local.ssh_key_path_by_provider, var.cloud_provider, null)
  selected_key_content = lookup(local.ssh_key_content_by_provider, var.cloud_provider, null)
  selected_network     = lookup(local.network_by_provider, var.cloud_provider, null)

  node_ips                = local.selected_node_ips == null ? {} : local.selected_node_ips
  node_ids                = local.selected_node_ids == null ? {} : local.selected_node_ids
  ssh_private_key_path    = local.selected_key_path == null ? "" : local.selected_key_path
  ssh_private_key_openssh = local.selected_key_content == null ? "" : local.selected_key_content
  network                 = local.selected_network

  # Danh sách node hoàn chỉnh: khung tĩnh từ node_plan + IP/ID từ module hạ tầng.
  nodes = {
    for name, node in local.node_plan : name => merge(node, {
      ip       = lookup(local.node_ips, name, "")
      id       = lookup(local.node_ids, name, "")
      ssh_user = var.ssh_user
    })
  }

  masters = { for name, node in local.nodes : name => node if node.role == "master" }
  workers = { for name, node in local.nodes : name => node if node.role == "worker" }

  master_ip = length(local.masters) > 0 ? values(local.masters)[0].ip : ""
}
