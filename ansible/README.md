# KubeClass - Unified Kubernetes & Sysbox Lab Deployment (Ansible)

Hệ thống triển khai tự động hóa hạ tầng Kubernetes chuyên dụng cho KubeClass Lab (hỗ trợ hai nền tảng phân phối chính: **Kubeadm v1.35** và **K3s**), tích hợp sẵn trình điều khiển container cách ly **Sysbox Runtime** (cho phép chạy Docker-in-Docker và Systemd với quyền root bên trong máy ảo Pod).

---

## 1. Kiến Trúc Hợp Nhất (Unified Multi-Distro Layout)
```
ansible/
├── ansible.cfg              # Cấu hình Ansible chung, liên kết inventory và roles
├── cluster.yml              # Playbook hợp nhất toàn cụm (Mặc định: Kubeadm)
├── README.md                # Tài liệu hướng dẫn sử dụng và chuyển đổi engine
├── inventory/
│   └── lab-cluster/
│       ├── hosts.ini        # Danh sách IP máy chủ Control Plane (Master) và Worker
│       ├── group_vars/      # Biến môi trường chung của cụm
│       └── host_vars/       # Biến riêng cho từng máy chủ (nếu có)
├── molecule/                # Bộ test tự động kiểm tra cú pháp và độ ổn định role
└── roles/                   # Tổ hợp Role hợp nhất cho toàn bộ hệ thống
    ├── common/              # Tinh chỉnh OS tiền kiểm: cgroup v2, HWE kernel, tắt swap, sysctl
    ├── containerd/          # Cài đặt và bật SystemdCgroup cho containerd (Kubeadm)
    ├── kubeadm-repo/        # Tải GPG Key và cài đặt kubelet, kubeadm, kubectl v1.35
    ├── kubeadm-master/      # Khởi tạo Kubeadm Control Plane & CNI Calico v3.30.0
    ├── kubeadm-worker/      # Tự động đồng bộ token và kết nạp Worker Nodes vào Kubeadm
    ├── k3s-master/          # Khởi tạo Control Plane cho K3s (Tùy chọn nhẹ)
    ├── k3s-worker/          # Kết nạp Worker Nodes vào K3s
    └── sysbox/              # Triển khai Sysbox Runtime & RuntimeClass (Dùng chung)
```

---

## 2. Các Tinh Chỉnh Tự Động Hóa Hệ Điều Hành (Preflight Auto-Tuning)
Khi chạy playbook, Role `common` sẽ tự động thực hiện các tác vụ can thiệp sâu vào HĐH để đảm bảo tiêu chuẩn vận hành Sysbox và Kubernetes:
1. **Nâng cấp Kernel HWE (>= 5.7):** Tự động phát hiện Ubuntu 20.04/22.04 có Kernel cũ (< 5.7) và cài đặt `linux-generic-hwe-20.04` để hỗ trợ tính năng `seccomp notify` của Sysbox, sau đó tự động reboot.
2. **Ép buộc sử dụng CGroup v2 (`cgroup2fs`):** Kiểm tra lệnh `stat -fc %T /sys/fs/cgroup`, nếu đang là `tmpfs` (cgroup v1), Ansible sẽ tự động sửa cấu hình GRUB (`GRUB_CMDLINE_LINUX="systemd.unified_cgroup_hierarchy=1"`), `update-grub` và khởi động lại node.
3. **Tắt Swap vĩnh viễn:** Thực hiện `swapoff -a` và comment toàn bộ dòng mount swap trong `/etc/fstab`.
4. **Load Kernel Modules & Sysctl:** Tải vĩnh viễn các module `overlay`, `br_netfilter`, `shiftfs` và áp dụng cờ `kernel.unprivileged_userns_clone = 1`, `ip_forward = 1` vào `/etc/sysctl.d/`.

---

## 3. Hướng Dẫn Kê Khai Danh Sách Máy Chủ (`hosts.ini`)
Mở tệp `inventory/lab-cluster/hosts.ini` và cấu hình địa chỉ IP máy chủ của bạn (chọn các tên hostname khớp và không bị lặp):

```ini
[k8s_master]
192.168.123.124 ansible_user=ubuntu

[k8s_workers]
192.168.123.125 ansible_user=ubuntu
192.168.123.126 ansible_user=ubuntu

[k8s_cluster:children]
k8s_master
k8s_workers
```

---

## 4. Hướng Dẫn Thực Thi Triển Khai (Deployment Commands)

### 🟢 Cách 1: Triển khai tiêu chuẩn với Kubeadm v1.35 & Calico (Mặc Định)
Playbook được thiết kế **mặc định chọn Kubeadm** cùng bộ trình điều khiển Containerd độc lập và mạng Calico v3.30.0:

```bash
cd ansible
ansible-playbook cluster.yml
```

### 🔵 Cách 2: Triển khai linh hoạt với K3s (Tùy Chọn Nhẹ / Edge)
Nếu máy chủ Lab có dung lượng khiêm tốn hoặc bạn muốn triển khai bằng K3s, chỉ cần truyền biến `-e kubernetes_distro=k3s` vào lệnh chạy:

```bash
cd ansible
ansible-playbook cluster.yml -e kubernetes_distro=k3s
```

### ⚡ Cách 3: Thực thi Playbook đơn trực tiếp không cần Clone Code (Zero-Clone / Standalone Playbook)
Đối với các kịch bản khởi tạo nhanh máy ảo (Cloud-init / User-data) hoặc thi hành các playbook file đơn (single-file / self-contained), bạn có thể dùng `curl` lấy trực tiếp nội dung từ kho chứa và truyền thẳng vào `ansible-playbook` mà **không cần `git clone`** toàn bộ dự án về máy:

#### 1. Thực thi trên máy sở tại (Localhost / Cloud-init qua `stdin`):
```bash
curl -fsSL https://raw.githubusercontent.com/ngtukien/sandbox-operator/main/ansible/<ten-playbook-don>.yml | ansible-playbook -i "localhost," -c local /dev/stdin
```

#### 2. Thực thi cho các máy chủ Remote (qua Process Substitution):
```bash
ansible-playbook -i "192.168.123.124,192.168.123.125," -u ubuntu <(curl -fsSL https://raw.githubusercontent.com/ngtukien/sandbox-operator/main/ansible/<ten-playbook-don>.yml)
```
> 💡 **Lưu ý kỹ thuật:** Phương pháp `curl ... | ansible-playbook` chỉ áp dụng lý tưởng cho **playbook đơn lẻ không bị ràng buộc đường dẫn cục bộ**. Khi muốn chạy trọn bộ cụm (`cluster.yml` yêu cầu nạp thư mục `roles/`) trên localhost mà không muốn thao tác `git clone` thủ công, bạn hãy xuất nhanh inventory tạm và chạy bằng `ansible-pull`:
> ```bash
> printf "[k8s_master]\nlocalhost ansible_connection=local\n\n[k8s_cluster:children]\nk8s_master\n" > /tmp/local.ini && \
> ansible-pull -U https://github.com/ngtukien/sandbox-operator.git -d /tmp/kubeclass-ansible \
>   -i /tmp/local.ini ansible/cluster.yml -e "kubernetes_distro=k3s"
> ```

---

## 5. Kiểm Chứng Sau Cài Đặt (Verification & Test Pod)

Sau khi Playbook hoàn tất 100%, hãy SSH vào Master Node và kiểm tra các dịch vụ nền tảng:

```bash
# 1. Kiểm tra trạng thái các Nodes (Phải ở trạng thái Ready và có nhãn sysbox-install=yes)
kubectl get nodes -o wide

# 2. Kiểm tra danh sách RuntimeClass
kubectl get runtimeclass
# Kết quả mong muốn:
# NAME          HANDLER       AGE
# sysbox-runc   sysbox-runc   5m

# 3. Kiểm tra pod quản trị Sysbox
kubectl get pods -n kube-system | grep -i sysbox
```

### Chạy thử nghiệm một máy ảo Pod mẫu với Sysbox Runtime:
Tạo file `test-vm.yaml` với nội dung sau:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: sysbox-test-vm
spec:
  runtimeClassName: sysbox-runc
  hostUsers: false
  nodeSelector:
    sysbox-install: "yes"
  containers:
  - name: ubuntu-vm
    image: registry.nestybox.com/nestybox/ubuntu-focal-systemd-docker:latest
    command: ["/sbin/init"]
  restartPolicy: Never
```
Thực thi và truy cập vào máy ảo:
```bash
kubectl apply -f test-vm.yaml
kubectl get pod sysbox-test-vm

# Khi Pod vào trạng thái Running, truy cập trực tiếp vào Terminal của máy ảo:
kubectl exec -it sysbox-test-vm -- /bin/bash

# Kiểm tra Docker bên trong Pod (Docker-in-Docker):
docker run hello-world
systemctl status
```
Nếu các lệnh trên thành công, hạ tầng Kubernetes KubeClass của bạn đã hoàn toàn sẵn sàng!
