terraform {
  # >= 1.5 để dùng được `check`/`precondition` và cú pháp for_each trên module.
  required_version = ">= 1.5.0"

  required_providers {
    libvirt = {
      source = "dmacvicar/libvirt"
      # Ghim nhánh 0.8.x: từ 0.9.0 provider được viết lại với schema map thẳng
      # XML libvirt (domain cần type/os/devices...), không tương thích cấu hình này.
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
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}
