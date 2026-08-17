# ==============================================================================
# PROVIDER CONFIGURATION
# ==============================================================================
# Provider libvirt chỉ thực sự kết nối tới hypervisor khi có resource libvirt_*
# được tạo (tức khi cloud_provider = "qemu"). Với các provider mây khác, module
# qemu có count = 0 nên khối provider này nằm im, không gây lỗi kết nối.
# ==============================================================================

provider "libvirt" {
  uri = var.libvirt_uri
}
