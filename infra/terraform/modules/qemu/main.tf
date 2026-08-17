# ==============================================================================
# MÁY ẢO QEMU/KVM QUA LIBVIRT
# ==============================================================================
# Luồng: cloud image (base volume, tải 1 lần) -> volume riêng cho từng node
# (copy-on-write từ base) -> seed ISO cloud-init -> domain (VM).
# ==============================================================================

# ------------------------------------------------------------------------------
# 1. BASE IMAGE: tải/nạp cloud image vào pool một lần, dùng chung cho mọi node
# ------------------------------------------------------------------------------
resource "libvirt_volume" "base" {
  name   = "${var.cluster_name}-base.qcow2"
  pool   = var.storage_pool
  source = var.os_image_source
  format = "qcow2"
}

# ------------------------------------------------------------------------------
# 2. ĐĨA HỆ ĐIỀU HÀNH CỦA TỪNG NODE (copy-on-write từ base image)
# ------------------------------------------------------------------------------
resource "libvirt_volume" "node" {
  for_each = local.nodes

  name           = "${each.key}.qcow2"
  pool           = var.storage_pool
  base_volume_id = libvirt_volume.base.id
  format         = "qcow2"
  size           = var.disk_size_gb * 1024 * 1024 * 1024
}

# ------------------------------------------------------------------------------
# 3. SEED ISO CLOUD-INIT (user-data + network-config + meta-data)
# ------------------------------------------------------------------------------
resource "libvirt_cloudinit_disk" "node" {
  for_each = local.nodes

  name = "${each.key}-seed.iso"
  pool = var.storage_pool

  user_data = templatefile("${path.module}/templates/cloud-init.yaml.tftpl", {
    hostname       = each.key
    domain         = local.domain_suffix
    cluster_name   = var.cluster_name
    ssh_user       = var.ssh_user
    ssh_public_key = local.ssh_public_key
  })

  network_config = templatefile("${path.module}/templates/network-config.yaml.tftpl", {
    mac         = each.value.mac
    ip          = each.value.ip
    prefix      = local.network_prefix
    gateway     = local.network_gateway
    domain      = local.domain_suffix
    nameservers = join(", ", concat([local.network_gateway], var.dns_forwarders))
  })

  meta_data = yamlencode({
    "instance-id"    = "${each.key}-${each.value.role}"
    "local-hostname" = each.key
  })
}

# ------------------------------------------------------------------------------
# 4. DOMAIN (VM)
# ------------------------------------------------------------------------------
resource "libvirt_domain" "node" {
  for_each = local.nodes

  name      = each.key
  memory    = each.value.memory_mb
  vcpu      = each.value.vcpu
  machine   = "q35"
  running   = true
  autostart = var.autostart_vms

  # qemu_agent chỉ để Terraform/virsh đọc thông tin VM; IP vẫn do cloud-init đặt tĩnh.
  qemu_agent = true

  cloudinit = libvirt_cloudinit_disk.node[each.key].id

  # host-passthrough cần cho k3s + Sysbox (kernel trong VM cần đủ CPU feature).
  cpu {
    mode = "host-passthrough"
  }

  network_interface {
    network_name = local.network_name
    mac          = each.value.mac

    # Không khai báo `addresses`/`hostname`: IP tĩnh do netplan trong VM đặt, nên
    # không cần DHCP reservation (và cũng để dùng được network không do Terraform
    # quản lý). Vì vậy cũng không phải chờ lease.
    wait_for_lease = false
  }

  disk {
    volume_id = libvirt_volume.node[each.key].id
  }

  # Console serial để debug bằng `virsh console <ten-vm>` khi SSH chưa lên.
  console {
    type        = "pty"
    target_port = "0"
    target_type = "serial"
  }

  console {
    type        = "pty"
    target_port = "1"
    target_type = "virtio"
  }

  graphics {
    type        = "spice"
    listen_type = "address"
    autoport    = true
  }

  depends_on = [libvirt_network.cluster]
}
