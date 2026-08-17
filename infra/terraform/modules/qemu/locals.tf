# ==============================================================================
# GÁN MAC VÀ IP TĨNH CHO TỪNG NODE
# ==============================================================================
# IP được tính trước bằng cidrhost() thay vì chờ DHCP lease. Lý do:
#   - Inventory Ansible và kubeconfig cần biết IP master ngay lúc plan.
#   - wait_for_lease của provider phụ thuộc lease/guest-agent nên hay treo.
# MAC deterministic (theo role + index) để netplan trong VM khớp đúng card mạng
# qua `match.macaddress`, không phụ thuộc tên interface (enp1s0/eth0...).
#
# Quy ước địa chỉ: master = .11, .12... | worker = .21, .22...
# ==============================================================================

locals {
  network_name = var.create_network ? (var.network_name != "" ? var.network_name : "${var.cluster_name}-net") : var.existing_network_name

  domain_suffix = "${var.cluster_name}.local"

  network_prefix  = tonumber(split("/", var.network_cidr)[1])
  network_gateway = cidrhost(var.network_cidr, 1)

  role_offsets = {
    master = 10
    worker = 20
  }

  role_mac_slot = {
    master = 1
    worker = 2
  }

  nodes = {
    for name, node in var.nodes : name => merge(node, {
      ip  = cidrhost(var.network_cidr, local.role_offsets[node.role] + node.index)
      mac = format("52:54:00:a1:%02x:%02x", local.role_mac_slot[node.role], node.index)
    })
  }
}
