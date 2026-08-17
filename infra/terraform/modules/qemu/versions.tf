terraform {
  required_version = ">= 1.5.0"

  required_providers {
    libvirt = {
      source = "dmacvicar/libvirt"
      # Xem ghi chú ở ../../versions.tf: nhánh 0.9.x đổi schema, không dùng được.
      version = "~> 0.8.3"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.4"
    }
  }
}
