# ==============================================================================
# MẠNG NAT CHO CỤM
# ==============================================================================
# Tạo một network riêng (mặc định <cluster_name>-net / 192.168.126.0/24) để
# không đụng tới network `default` hay `k3s-net` đã có trên máy. DHCP vẫn bật
# cho tiện debug, nhưng node dùng IP tĩnh do cloud-init đặt.
# ==============================================================================

resource "libvirt_network" "cluster" {
  count = var.create_network ? 1 : 0

  name      = local.network_name
  mode      = "nat"
  domain    = local.domain_suffix
  addresses = [var.network_cidr]
  autostart = true

  dhcp {
    enabled = true
  }

  dns {
    enabled    = true
    local_only = false
  }
}
