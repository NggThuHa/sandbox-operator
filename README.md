# 🚀 KubeEdu Systemd Operator — Cloud-Native Practical Lab & Sandbox Platform

![Kubebuilder](https://img.shields.io/badge/Kubebuilder-v4-blue.svg) ![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.32+-326ce5.svg) ![Go Version](https://img.shields.io/badge/Go-v1.26+-00ADD8.svg) ![Ansible](https://img.shields.io/badge/Ansible-Kubespray%20Style-EE0000.svg)

**KubeEdu Systemd Operator** là bộ điều khiển Kubernetes (Kubernetes Operator) tiêu chuẩn doanh nghiệp của hệ sinh thái **KubeEdu**, chuyên tự động hóa việc khởi tạo và quản trị nền tảng phòng thí nghiệm & Sandbox (Cloud-Native Lab Platform) trên điện toán đám mây, phục vụ tối ưu cho mục đích đào tạo thực thao (University Practical Labs, Coding Bootcamps, DevSecOps Cyber-Range). 

Hệ thống tích hợp sâu với **Sysbox Runtime** để vận hành các máy ảo Container (System Containers / Docker-in-Docker với đầy đủ `systemd` gốc) ngay bên trong Kubernetes Pod với hiệu năng cao nhạy bén như máy chủ vật lý, hỗ trợ trọn vẹn kết nối Terminal/VSCode qua Web (WebSocket Secure).

---

## 🏛️ Kiến Trúc Hệ Thống & Tài Nguyên Tùy Chỉnh (CRDs)

Hệ thống hoạt động dựa trên 2 thực thể Custom Resources chính:

```mermaid
graph TD
    User([🧑‍🎓 Sinh viên / Giảng viên]) -->|Gõ YAML hoặc qua API Web| Operator[⚙️ KubeEdu Systemd Operator]
    Operator -->|Quản trị Không gian Lab| VC[📦 VirtualCluster CRD]
    Operator -->|Quản trị Máy Ảo Lab| VI[🖥️ VirtualInstance CRD]

    subgraph "Kubernetes Namespace (Isolated by VirtualCluster)"
        VC -->|1. Cấp phát| NS((K8s Namespace))
        VC -->|2. Giám sát Giới hạn| RQ[⚖️ ResourceQuota<br/>CPU / Mem / Storage]
        VC -->|3. Bảo mật Cách ly| NP[🛡️ NetworkPolicy<br/>Block Ingress / Open Egress]
        
        VI -->|4. Khởi tạo| Pod[🚀 Sysbox Pod<br/>RuntimeClass: sysbox-runc]
        VI -->|5. Lưu trữ Dữ liệu| PVC[💾 PersistentVolumeClaim<br/>Longhorn / Local Storage]
        VI -->|6. Gán Cổng TCP| SVC[🌐 ClusterIP Service]
    end

    SVC -->|7. Cấp Đên & Bảo mật SSL| ING[🔒 HTTPS Ingress<br/>Cert-Manager & WSS Terminal]
```

### 1. `VirtualCluster` — Cụm Phòng Lab Độc Lập
* **Cách ly vô song (Multi-Tenant Isolation):** Tự động sinh Kubernetes Namespace chuẩn RFC 1123 đi kèm với **NetworkPolicy** (Mặc định cho phép 100% kết nối Egress để sinh viên thoải mái `docker pull`, `git clone`, `apt install`, nhưng phong tỏa chặt chẽ Ingress để chống rò rỉ dữ liệu hay gian lận thi cử giữa các phòng lab).
* **Quản trị ranh giới tài nguyên (Resource Quotas):** Áp đặt khắt khe giới hạn tối đa CPU, RAM, Storage và số lượng Object thông qua bộ chuẩn `resource.Quantity` chính quy của Kubernetes.
* **Quy tắc Finalizer-First & Hết hạn thông minh (TTL Anti-Drift):** Cam kết đảm bảo dọn sạch sẽ tài nguyên khi xóa cụm thông qua OwnerReferences và tự động giải tán phòng Lab sau thời gian quy định (`ttl` như `4h`, `120m`) nhằm chống ùn đọng rác trên Cluster.

### 2. `VirtualInstance` — Máy Ảo Thực Thao Sysbox-Ready
* **Động cơ Sysbox (`sysbox-runc`):** Vận hành container như một hệ điều hành trọn vẹn (cho phép chạy systemd, Docker, Kubernetes K8s-in-K8s bên trong Pod mà không cần cờ đặc quyền nhạy cảm `privileged: true`).
* **Hạ tầng Lưu trữ & Truy Cập Động:** 
  * Tự động phán đoán và quy hoạch lớp lưu trữ (`StorageClass`) và lớp điều hướng (`IngressClass`) với 3 tầng ưu tiên: **Biến môi trường ENV > Tùy chỉnh trong Spec > Mặc định hệ thống**.
  * Tự động xuất xưởng Domain HTTPS đi kèm chứng chỉ TLS hợp lệ (Cert-Manager) và luồng băng thông **WSS (WebSocket Secure)** tương thích 100% với các dịch vụ Terminal Web-based (như `ttyd`, `code-server`).

---

## 🏗️ Hướng Dẫn Kích Hoạt Cụm Kubeadm/K3s & Sysbox (Ansible Kubespray-Style)

Hệ thống được trang bị sẵn bộ engine Ansible tối ưu hóa theo quy chuẩn phong cách **Kubespray**, giúp bạn biến hàng tá máy chủ thô (Bare-metal / VM) thành cụm Lab Kubeadm hoặc K3s chạy Sysbox chỉ trong vài phút!

### Cây Thư Mục Khởi Tạo Cụm (`ansible/`)
```text
ansible/
├── ansible.cfg                    # Cấu hình Ansible tối ưu SSH Pipelining & Callback
├── cluster.yml                    # 🚀 Playbook hợp nhất toàn cụm (Mặc định: Kubeadm v1.35)
├── README.md                      # 📜 Tài liệu hướng dẫn chuyên sâu chi tiết cho Ansible
├── inventory/                     # Kho Phân Cực Môi Trường (Environment Segmentation)
│   └── lab-cluster/               
│       ├── hosts.ini              # Danh sách IP máy chủ Master và Workers
│       └── group_vars/            # Kho cấu hình biến toàn bộ cụm (Kubeadm/K3s version, Sysbox)
└── roles/                         # Bộ động cơ tự động hóa chuyên sâu (common, containerd, kubeadm-*, k3s-*, sysbox)
```

### Cách Thực Thi Khởi Tạo & Quản Trị Cụm

#### 1. Triển khai chuẩn (Kubeadm v1.35 & Calico - Mặc định) hoặc K3s:
Cấu hình IP trong **`ansible/inventory/lab-cluster/hosts.ini`**, sau đó chạy:
```bash
cd ansible
# Chạy mặc định với Kubeadm:
ansible-playbook cluster.yml

# Chạy linh hoạt với K3s (Edge/Nhẹ):
ansible-playbook cluster.yml -e kubernetes_distro=k3s
```

#### 2. ⚡ Thực thi Playbook Đơn Trực Tiếp (Zero-Clone / Không cần tải repo):
Với các kịch bản cài đặt siêu nhanh (Cloud-init / User-data) hoặc thi hành các playbook cấu hình độc lập file đơn (single-file), bạn có thể dùng `curl` lấy trực tiếp YAML từ GitHub và truyền qua ống dẫn vào `ansible-playbook` mà **không cần chạy lệnh `git clone`**:
```bash
# Thực thi trực tiếp cho máy Local (qua stdin):
curl -fsSL https://raw.githubusercontent.com/KubeEdu/systemd-operator/main/ansible/<ten-playbook-don>.yml | ansible-playbook -i "localhost," -c local /dev/stdin

# Thực thi trực tiếp cho các máy chủ Remote (qua Process Substitution):
ansible-playbook -i "192.168.123.124,192.168.123.125," -u ubuntu <(curl -fsSL https://raw.githubusercontent.com/KubeEdu/systemd-operator/main/ansible/<ten-playbook-don>.yml)
```
*(Chi tiết tham khảo đầy đủ tại [ansible/README.md](file:///home/ngtukien/Documents/Kubebuilder/ansible/README.md))*


---

## 🛠️ Hướng Dẫn Phát Triển & Triển Khai Operator

### 1. Yêu cầu Tiền quyết (Prerequisites)
* **Go:** `v1.26.0+`
* **Docker:** `17.03+`
* **Kubernetes Cluster:** `v1.32+` (Cụm K3s tạo từ bước trên hoặc dàn Kind/Envtest để test local).

### 2. Kiểm Thử & Kiểm Soát Ngữ Pháp Mã Nguồn (Testing & Linting)
Trong quá trình bổ sung hoặc chỉnh sửa code Controller:
```bash
# Tự động định dạng lại code chuẩn và sửa lỗi lint:
make lint-fix

# Sinh lại các tệp CRD YAML và DeepCopy sau khi sửa đổi spec:
make manifests generate

# Kích hoạt bộ kiểm thử tự động Unit test qua Envtest (Ginkgo/Gomega):
make test
```

### 3. Đưa Operator Lên Cụm K3s Thực Chiến

#### Cách 1: Cài đặt Siêu Tốc qua GitHub Releases / Kustomize (Single-Command Install)
Dành cho người dùng cuối (End-users / Admin Cụm K8s), bạn có thể chạy duy nhất một lệnh bằng cách tải bundle `install.yaml` từ bản phát hành (Releases) mới nhất, hoặc nạp thẳng Kustomize từ mã nguồn đã pull bằng Ansible trước đó:
```bash
# Lựa chọn A (Từ gói GitHub Releases khi đã tạo tag chính thức v1.x.x):
kubectl apply -f https://github.com/KubeEdu/systemd-operator/releases/latest/download/install.yaml

# Lựa chọn B (Trực tiếp từ thư mục Ansible đã pull về máy trước đó):
kubectl apply -k /tmp/kubeedu-ansible/config/default
```
> 💡 *File `dist/install.yaml` được tự động sinh ra bằng lệnh `make build-installer IMG=...` và đính kèm vào trang **Releases** của GitHub mỗi khi bạn tạo và đẩy tag phiên bản mới (ví dụ: `git tag v1.0.0 && git push --tags`). Do được quản lý tự động bởi CI/CD nên thư mục `dist/` bị vô hiệu hóa theo dõi trong `.gitignore` của nhánh `main`.*

#### Cách 2: Triển khai cho Nhà Phát Triển từ Mã Nguồn (Developer Mode)
**Bước 1: Biên dịch và đẩy Docker Image của Operator lên Registry:**
```bash
export IMG="ghcr.io/kubeedu/systemd-operator:v1.0.0"
make docker-build docker-push IMG=$IMG
```

**Bước 2: Triển khai trực tiếp bằng Kustomize / Makefile:**
```bash
make install
make deploy IMG=$IMG
```

---

## 📦 Ví Dụ Kịch Bản Sử Dụng Nhanh (Quickstart Samples)

### 1. Khởi tạo một Cụm Phòng Lab (`VirtualCluster`) cho Sinh viên
Tạo file `sample-virtualcluster.yaml`:
```yaml
apiVersion: lab.devops.toiyeuptit.com/v1alpha1
kind: VirtualCluster
metadata:
  name: student-devops-lab01
  namespace: default
  labels:
    student_id: "sv-2026-001"
    lab_id: "lab-k8s-basics"
spec:
  ttl: "4h"            # Tự động dọn trọn gói phòng Lab sau 4 tiếng
  network:
    type: "external"   # 'external' (mở toang tiện dụng) hoặc 'internal' (cách ly thi cử)
  quota:
    compute:
      cpu:
        limit: "4"     # 4 vCPU
      memory:
        limit: "8Gi"   # 8GB RAM
    storage:
      localLimit: "50Gi"
      networkLimit: "20Gi"
    objects:
      podsLimit: 20
      servicesLimit: 10
```
Áp dụng lên K8s: `kubectl apply -f sample-virtualcluster.yaml`

---

### 2. Khởi tạo Máy Ảo Thực Thao (`VirtualInstance`) kèm Terminal Web
Tạo file `sample-virtualinstance.yaml`:
```yaml
apiVersion: lab.devops.toiyeuptit.com/v1alpha1
kind: VirtualInstance
metadata:
  name: ubuntu-sysbox-devbox
  namespace: default
spec:
  virtualClusterRef: student-devops-lab01
  image: "ubuntu:24.04"      # Image chuyên dụng cho môn học/thi cử
  runtimeClassName: "sysbox-runc"
  resources:
    cpu:
      request: "1"
      limit: "2"
    memory:
      request: "2Gi"
      limit: "4Gi"
  storage:
    root:
      size: "15Gi"
      type: local            # Ổ Root I/O cực cao từ local-path
    dataVolumes:
      - name: docker-data
        mountPath: /var/lib/docker
        size: "20Gi"
        type: network        # Ổ Dữ liệu lưu vết phân tán từ longhorn
  ports:
    - name: web-terminal
      port: 80
      targetPort: 7681       # Cổng ttyd terminal bên trong Pod
      expose: true           # Tự động cấp domain HTTPS WSS qua Cert-Manager
```
Áp dụng lên K8s: `kubectl apply -f sample-virtualinstance.yaml`

---

## 🔍 Khung Quan Sát Cấu Hình Môi Trường (Environment Variables)

Bộ điều khiển Operator có khả năng tinh chỉnh linh hoạt lớp lưu trữ (Storage Class) và bộ điều hướng (Ingress Controller) của Cụm thông qua các biến môi trường cấu hình tại Pod của Controller Manager (hoặc file `.env` / `.env.example`):

| Biến Môi Trường (`ENV_VAR`) | Giá Trị Mặc Định Khi Không Khai Báo | Ý Nghĩa & Vai Trò Trong Hệ Thống |
| :--- | :--- | :--- |
| `DEFAULT_LOCAL_STORAGE_CLASS` | `local-path` | Tên của StorageClass mặc định trong K3s dùng cho ổ cứng SSD Nội bộ. |
| `DEFAULT_NETWORK_STORAGE_CLASS` | `longhorn` | Tên của StorageClass phân tán, lưu trữ dữ liệu sinh viên bền vững lâu dài. |
| `DEFAULT_INGRESS_CLASS` | `nginx` | Trình điều hướng traffic mặc định, hỗ trợ tối đa WebSockets (WSS). |
| `DEFAULT_RUNTIME_CLASS` | `sysbox-runc` | Định danh runtime engine chạy container-in-container bên trong K8s. |

---

## 📜 Giám Sát Tự Động & Dọn Rác (Observability & Cleanup)

* Toàn bộ tiến độ giải ngân tài nguyên (`Conditions`) và lưu lượng đã dùng (`QuotaUsage`) đều được theo dõi thời gian thực:
  ```bash
  kubectl get virtualcluster,virtualinstance -A -o wide
  ```
* **Bảo đảm Dọn Sạch Sẽ (Garbage Collection):** Xóa `VirtualCluster` gốc sẽ phát đi tín hiệu qua Garbage Collector K8s tự động dọn sạch an toàn toàn bộ Pods, PVCs, NetworkPolicies và Services trong Namespace tương ứng!

---
*Phát triển bởi **Nguyễn Tự Kiên** (2026).*  
*Licensed under the Apache License, Version 2.0.*
