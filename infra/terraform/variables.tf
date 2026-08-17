# ==============================================================================
# BIẾN CHUNG (KHÔNG PHỤ THUỘC NỀN TẢNG)
# ==============================================================================
# Các biến ở đây đúng với mọi cloud_provider. Biến riêng của QEMU/libvirt nằm ở
# cuối file, trong khối "QEMU/LIBVIRT ONLY".
# ==============================================================================

variable "cluster_name" {
  type        = string
  default     = "kubeclass"
  description = "Tiền tố đặt tên cho mọi tài nguyên (VM, volume, network)."

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]{1,30}$", var.cluster_name))
    error_message = "cluster_name chỉ gồm chữ thường, số và dấu gạch ngang (2-31 ký tự)."
  }
}

# ------------------------------------------------------------------------------
# TOPOLOGY
# ------------------------------------------------------------------------------
variable "master_count" {
  type        = number
  default     = 1
  description = <<-EOT
    Số node control-plane. Mặc định 1: với k3s, node server cũng chạy pod nên
    một VM duy nhất đóng cả vai master + worker.
    Chỉ chấp nhận giá trị 1 vì role k3s-master/kubeadm-master hiện chưa hỗ trợ
    HA (chưa có --cluster-init cho embedded etcd).
  EOT

  validation {
    condition     = var.master_count == 1
    error_message = "master_count hiện chỉ hỗ trợ 1 (role Ansible chưa hỗ trợ control-plane HA)."
  }
}

variable "worker_count" {
  type        = number
  default     = 0
  description = "Số node worker bổ sung. Mặc định 0 (cụm single-node)."

  validation {
    condition     = var.worker_count >= 0 && var.worker_count <= 10
    error_message = "worker_count phải nằm trong khoảng 0-10."
  }
}

variable "master_vcpu" {
  type        = number
  default     = 4
  description = "Số vCPU cho node master."
}

variable "master_memory_mb" {
  type        = number
  default     = 6144
  description = "RAM (MB) cho node master. Cụm single-node chạy cả k3s + Sysbox + OpenEBS nên nên để >= 4096."
}

variable "worker_vcpu" {
  type        = number
  default     = 2
  description = "Số vCPU cho mỗi node worker."
}

variable "worker_memory_mb" {
  type        = number
  default     = 4096
  description = "RAM (MB) cho mỗi node worker."
}

variable "disk_size_gb" {
  type        = number
  default     = 60
  description = <<-EOT
    Dung lượng đĩa mỗi node (GB). Với QEMU đây là qcow2 sparse nên không chiếm
    thật ngay. Phải lớn hơn openebs_lvm_pool_size vì role openebs-lvm cắt file
    loopback ngay trên rootfs.
  EOT
}

# ------------------------------------------------------------------------------
# SSH
# ------------------------------------------------------------------------------
variable "ssh_user" {
  type        = string
  default     = "ubuntu"
  description = "User đăng nhập VM (được cloud-init tạo với sudo NOPASSWD)."
}

variable "ssh_public_key" {
  type        = string
  default     = ""
  description = <<-EOT
    Nội dung SSH public key nạp vào VM. Để rỗng thì Terraform tự sinh một cặp
    khoá ed25519 và lưu khoá riêng vào .ssh/ trong thư mục này.
  EOT
}

variable "ssh_private_key_path" {
  type        = string
  default     = ""
  description = <<-EOT
    Đường dẫn khoá riêng tương ứng với ssh_public_key (chỉ cần khi bạn tự cấp
    khoá). Để rỗng thì dùng ssh-agent, hoặc khoá do Terraform tự sinh.
  EOT
}

# ------------------------------------------------------------------------------
# ANSIBLE
# ------------------------------------------------------------------------------
variable "run_ansible" {
  type        = bool
  default     = true
  description = "true: tự chạy infra/ansible/cluster.yml sau khi VM sẵn sàng. false: chỉ tạo VM + sinh inventory."
}

variable "k3s_version" {
  type        = string
  default     = "v1.32.5+k3s1"
  description = "Phiên bản k3s cài lên node (khớp group_vars/all/all.yml của Ansible)."
}

variable "openebs_lvm_pool_size" {
  type        = string
  default     = "20G"
  description = <<-EOT
    Dung lượng file loopback mà role openebs-lvm dùng để tạo volume group
    'openebs-vg'. Mặc định của role là 50G — quá lớn so với đĩa 60GB nên ở đây
    hạ xuống 20G.
  EOT
}

variable "ansible_skip_tags" {
  type        = list(string)
  default     = []
  description = <<-EOT
    Danh sách tag bị bỏ qua khi chạy playbook. Ví dụ để dựng cụm nhanh, bỏ
    Sysbox và OpenEBS: ["sysbox", "storage"].
  EOT
}

variable "ansible_extra_vars" {
  type        = map(string)
  default     = {}
  description = "Extra vars bổ sung truyền cho ansible-playbook (ghi đè các giá trị mặc định)."
}

variable "ansible_extra_args" {
  type        = string
  default     = ""
  description = "Tham số dòng lệnh thêm cho ansible-playbook, ví dụ \"-vv\" hoặc \"--tags common\"."
}

# ==============================================================================
# QEMU/LIBVIRT ONLY
# ==============================================================================
variable "libvirt_uri" {
  type        = string
  default     = "qemu:///system"
  description = "URI kết nối libvirt. Dùng qemu:///session nếu không có quyền vào libvirtd hệ thống."
}

variable "storage_pool" {
  type        = string
  default     = "default"
  description = "Tên storage pool của libvirt để chứa volume và seed ISO."
}

variable "os_image_source" {
  type        = string
  default     = "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
  description = <<-EOT
    Nguồn cloud image cho VM. Có thể là URL hoặc đường dẫn file cục bộ (đã tải
    sẵn) để không phải tải lại. Ansible chỉ hỗ trợ họ Debian/Ubuntu.
  EOT
}

variable "create_network" {
  type        = bool
  default     = true
  description = "true: Terraform tạo network NAT riêng. false: dùng network libvirt có sẵn (existing_network_name)."
}

variable "network_name" {
  type        = string
  default     = ""
  description = "Tên network do Terraform tạo. Để rỗng sẽ dùng \"<cluster_name>-net\"."
}

variable "network_cidr" {
  type        = string
  default     = "192.168.126.0/24"
  description = <<-EOT
    Dải mạng NAT của cụm. Mặc định 192.168.126.0/24 để không trùng network
    libvirt sẵn có trên máy (default 192.168.122.0/24, k3s-net 192.168.123.0/24).
  EOT

  validation {
    condition     = can(cidrhost(var.network_cidr, 1))
    error_message = "network_cidr phải là CIDR IPv4 hợp lệ, ví dụ 192.168.126.0/24."
  }
}

variable "existing_network_name" {
  type        = string
  default     = ""
  description = "Tên network libvirt có sẵn, dùng khi create_network = false."
}

variable "dns_forwarders" {
  type        = list(string)
  default     = ["1.1.1.1", "8.8.8.8"]
  description = "DNS mà VM sử dụng (ngoài DNS nội bộ của libvirt)."
}

variable "autostart_vms" {
  type        = bool
  default     = false
  description = "true: VM tự khởi động cùng libvirtd sau khi reboot host."
}
