# KubeClass Infrastructure — Terraform (QEMU/KVM) + Ansible k3s

Bộ Terraform tạo máy ảo qua **QEMU/KVM (libvirt)** rồi tự động gọi playbook
[`../ansible/cluster.yml`](../ansible/cluster.yml) để dựng cụm **k3s + Sysbox + OpenEBS LVM**.
Chỉ một lệnh `terraform apply` là có cụm chạy được; `terraform destroy` là xoá sạch.

Mặc định: **1 VM Ubuntu 24.04 đóng cả vai master + worker** (k3s server vẫn schedule pod bình thường).

---

## 1. Yêu cầu trên máy host

| Thành phần | Kiểm tra |
|---|---|
| Terraform >= 1.5 | `terraform version` |
| libvirt + QEMU/KVM | `virsh --connect qemu:///system version` |
| `genisoimage` hoặc `mkisofs` | dùng để tạo seed ISO cloud-init |
| Ansible | `ansible-playbook --version` |
| Quyền | user thuộc nhóm `libvirt` và `kvm` (`groups`) |
| Storage pool | `virsh --connect qemu:///system pool-list` phải có pool `default` đang active |

Nếu thiếu (Debian/Ubuntu):

```bash
sudo apt install -y qemu-kvm libvirt-daemon-system libvirt-clients genisoimage ansible
sudo usermod -aG libvirt,kvm "$USER"   # cần đăng xuất/đăng nhập lại
```

---

## 2. Sử dụng

```bash
cd infra/terraform

cp terraform.tfvars.example terraform.tfvars   # tuỳ chọn, mặc định đã chạy được

terraform init
terraform plan
terraform apply
```

Lần apply đầu mất khoảng 10–20 phút: tải cloud image (~600MB), khởi tạo VM, rồi chạy trọn 5 phase của
`cluster.yml` (chuẩn hoá OS → OpenEBS LVM → k3s → containerd cho Sysbox → Sysbox RuntimeClass).

Sau khi xong:

```bash
terraform output next_steps

export KUBECONFIG="$(terraform output -raw kubeconfig_path)"
kubectl get nodes -o wide      # Ready, có label sysbox-install=yes
kubectl get runtimeclass       # sysbox-runc
kubectl get sc                 # openebs-lvm
```

SSH vào node:

```bash
terraform output ssh_commands
```

Xoá toàn bộ (VM, đĩa, network, inventory, kubeconfig):

```bash
terraform destroy
```

---

## 3. Lựa chọn nền tảng — [`platform.tf`](platform.tf)

Hai biến ở file riêng để dễ tìm và dễ mở rộng:

```hcl
cloud_provider = "qemu"   # qemu (đã hỗ trợ) | aws | azure | gcp (chưa implement)
k8s_distro     = "k3s"    # k3s | kubeadm
```

```bash
# Dựng cụm kubeadm + Calico thay vì k3s (Ansible đã có sẵn roles)
terraform apply -var k8s_distro=kubeadm
```

Chọn provider chưa có module thì Terraform báo lỗi rõ ràng ngay ở bước plan
(`null_resource.platform_guard`) thay vì fail giữa đường.

### Thêm provider mây mới

1. Tạo `modules/<provider>/` trả về đúng **output contract**:
   | Output | Ý nghĩa |
   |---|---|
   | `node_ips` | map `tên node => IP` (key phải trùng key của biến `nodes` truyền vào) |
   | `node_ids` | map `tên node => id tài nguyên` (tạo dependency cho provisioner) |
   | `ssh_private_key_path` | đường dẫn khoá riêng cho Ansible |
   | `ssh_private_key_openssh` | nội dung khoá (sensitive), cho `remote-exec` |
   | `network` | thông tin mạng (hoặc `null`) |
2. Bỏ comment khối `module "<provider>"` trong [`main.tf`](main.tf) và thêm vào `local.*_by_provider`.
3. Thêm tên vào `local.implemented_providers` trong [`platform.tf`](platform.tf).

[`ansible.tf`](ansible.tf) và [`outputs.tf`](outputs.tf) **không cần sửa** — chúng chỉ đọc `local.nodes`.
Khung danh sách node (`local.node_plan` trong [`locals.tf`](locals.tf)) được tính ở root nên `for_each`
luôn xác định được ở bước plan, kể cả khi IP chỉ biết sau apply (trường hợp AWS/Azure).

---

## 4. Các biến hay dùng

| Biến | Mặc định | Ghi chú |
|---|---|---|
| `cluster_name` | `kubeclass` | tiền tố mọi tài nguyên |
| `master_count` | `1` | chỉ nhận 1 — role Ansible chưa hỗ trợ control-plane HA |
| `worker_count` | `0` | tăng lên là thêm VM vào group `k8s_workers` |
| `master_vcpu` / `master_memory_mb` | `4` / `6144` | cụm single-node chạy cả Sysbox + OpenEBS |
| `disk_size_gb` | `60` | qcow2 sparse, phải lớn hơn `openebs_lvm_pool_size` |
| `network_cidr` | `192.168.126.0/24` | master nhận `.11`, worker nhận `.21`, `.22`… |
| `os_image_source` | Ubuntu 24.04 cloud image | trỏ file cục bộ để không tải lại |
| `ssh_public_key` | `""` | rỗng ⇒ Terraform tự sinh ed25519 vào `./.ssh/` |
| `run_ansible` | `true` | `false` ⇒ chỉ tạo VM và sinh inventory |
| `openebs_lvm_pool_size` | `20G` | ghi đè mặc định 50G của role `openebs-lvm` |
| `ansible_skip_tags` | `[]` | ví dụ `["sysbox","storage"]` để dựng cụm nhanh |

Xem đầy đủ trong [`variables.tf`](variables.tf) và [`terraform.tfvars.example`](terraform.tfvars.example).

Dùng network libvirt sẵn có thay vì tạo mới:

```hcl
create_network        = false
existing_network_name = "k3s-net"
network_cidr          = "192.168.123.0/24"   # phải khớp dải của network đó
```

---

## 5. Cách phần Ansible được gọi

1. `local_file.inventory` ghi inventory ra
   `../ansible/inventory/lab-cluster/tf-hosts.ini` — đặt cạnh `group_vars/` để Ansible tự nạp
   [`group_vars/all/all.yml`](../ansible/inventory/lab-cluster/group_vars/all/all.yml) và
   [`group_vars/k8s_cluster.yml`](../ansible/inventory/lab-cluster/group_vars/k8s_cluster.yml).
   File `hosts.ini` gốc **không bị ghi đè**.
2. `null_resource.wait_cloudinit` SSH vào từng node chờ `cloud-init status --wait` (tránh đụng apt lock).
3. `null_resource.ansible` chạy:

   ```bash
   cd infra/ansible
   ansible-playbook -i inventory/lab-cluster/tf-hosts.ini cluster.yml \
     -e '{"kubernetes_distro":"k3s","k3s_version":"...","openebs_lvm_pool_size":"20G",
          "real_user":"ubuntu","real_home":"/home/ubuntu"}'
   ```

> **Vì sao phải truyền `real_user`/`real_home`:** role
> [`k3s-master`](../ansible/roles/k3s-master/tasks/main.yaml) lấy `USER`/`HOME` của **máy chạy Ansible** rồi
> `chown` thư mục `~/.kube` trên **VM**. Nếu không ghi đè, task fail vì user của host không tồn tại trong VM.
> Extra vars có precedence cao nhất nên ghi đè được `set_fact` — nhờ vậy không phải sửa role nào.

Chạy lại Ansible mà không dựng lại VM:

```bash
terraform output -raw ansible_command      # in ra lệnh đầy đủ
# hoặc buộc Terraform chạy lại provisioner:
terraform apply -replace='null_resource.ansible[0]'
```

Terraform tự chạy lại playbook khi `cluster.yml`, `roles/**` hoặc `group_vars/**` thay đổi (theo hash nội dung).

---

## 6. Xử lý sự cố

| Hiện tượng | Cách xử lý |
|---|---|
| `error connecting to libvirt: permission denied` | user chưa ở nhóm `libvirt`, hoặc dùng `libvirt_uri = "qemu:///session"` |
| Provisioner treo ở `Still creating...` | VM chưa lên mạng: `virsh --connect qemu:///system console <tên-vm>` để xem log boot |
| `Could not open '/var/lib/libvirt/images/...': Permission denied` | pool `default` bị đổi quyền; `sudo chmod 0711 /var/lib/libvirt/images` |
| Ansible fail giữa đường | sửa role rồi `terraform apply` lại — chỉ provisioner Ansible chạy lại, VM giữ nguyên |
| Muốn cụm nhanh, bỏ Sysbox | `terraform apply -var 'ansible_skip_tags=["sysbox","storage"]'` |
| Provider libvirt 0.9.x | đã ghim `~> 0.8.3`: từ 0.9.0 provider đổi hoàn toàn schema (`libvirt_domain` cần `type`/`os`/`devices`) |
| Network `192.168.126.0/24` bị trùng | đổi `network_cidr`, IP node tự tính lại theo dải mới |

---

## 7. Cấu trúc

```
infra/terraform/
├── platform.tf                 # ★ cloud_provider + k8s_distro + guard
├── locals.tf                   # node_plan: khung node tĩnh (tên, role, spec)
├── main.tf                     # dispatch module theo provider + gom output
├── variables.tf                # biến chung + nhóm riêng cho QEMU/libvirt
├── ansible.tf                  # inventory + chờ cloud-init + chạy cluster.yml
├── outputs.tf, versions.tf, providers.tf
├── templates/hosts.ini.tftpl   # mẫu inventory Ansible
└── modules/qemu/               # toàn bộ phần libvirt
    ├── main.tf                 # base volume → volume node → seed ISO → domain
    ├── network.tf              # network NAT
    ├── ssh.tf                  # tự sinh khoá ed25519
    ├── locals.tf               # gán IP tĩnh + MAC deterministic
    ├── outputs.tf              # output contract
    └── templates/              # cloud-init user-data + netplan
```
