# 1. API Architecture
## 1.1. ClusterLab (Cụm máy ảo DevOps Lab)
- Định nghĩa Custom Resource Definition (CRD):
```yaml
apiVersion: lab.devops.toiyeuptit.com/v1alpha1
kind: ClusterLab
metadata:
  name: sample-clusterlab
  labels:
    student_id: "student-01"
    session_id: "session-01"
    lab_id: "lab-01"
spec:
  # Thời gian sống của cụm Lab (TTL - Time To Live) sau khi tạo
  # Định dạng thời gian tiêu chuẩn, ví dụ: "4h", "120m", "2h30m". Sau thời gian này, cụm và mọi máy ảo bên trong sẽ tự động được thu hồi!
  ttl: "4h"

  # Cấu hình Mạng tổng thể của cả Namespace
  network:
    # isolate: Chặn mọi traffic Ingress
    # internal: Các InstanceLab gọi được nhau nội bộ trong Namespace, chặn ngoài
    # external: Mở Ingress/LB cho phép bên ngoài truy cập vào
    type: "external" 

  # Giới hạn tài nguyên (Quota) cho toàn bộ cụm
  quota:
    storage:
      # Tổng limits.ephemeral-storage của tất cả container trong namespace không vượt quá 30Gi
      # Với mỗi InstanceLab khai báo storage.limit: 20Gi
      # → Namespace này tối đa chứa 1 InstanceLab (20Gi < 30Gi), từ thứ 2 sẽ bị từ chối
      limit: "30Gi"
    
    compute:
      cpu:
        limit: "4"            # Tổng CPU không vượt quá 4 cores
      memory:
        limit: "8Gi"          # Tổng RAM không vượt quá 8GB
    
    objects:
      podsLimit: 15           # Giới hạn số lượng Pod thật trên Namespace này
      servicesLimit: 5        # Giới hạn số lượng Service
```
- Status (Trạng thái được Operator tự động tổng hợp & trả về cho UI/Frontend):
```yaml
status:
  # Tình trạng hoạt động tổng thể của cụm
  # Phase có thể là: Pending | Provisioning | Running | Degraded | Failed | Terminating
  phase: "Running"
  
  # Thời điểm hết hạn chính xác của cụm Lab (được tính từ CreationTimestamp + spec.ttl)
  # Giao diện Web có thể dùng trường này để hiển thị đồng hồ đếm ngược cho học viên!
  expiresAt: "2026-08-02T23:20:00Z"

  # Tên K8s Namespace thực tế mà Operator đã khởi tạo cho ClusterLab này bên dưới (không vượt quá 63 ký tự)
  targetNamespace: "sample-clusterlab-x7z9a"
  
  # Số lượng máy ảo (InstanceLab) đang gắn vào cụm này
  instanceCount:
    total: 2
    ready: 2
  
  # Thống kê sử dụng thực tế (Real-time Quota Usage) đong đếm từ K8s ResourceQuota
  # Cực kỳ hữu ích để UI hiển thị thanh phần trăm (%) tài nguyên cho Quản trị viên/Học viên
  quotaUsage:
    compute:
      cpuUsed: "1500m"     # Đã xài 1.5 core / Limit: 4 core
      memoryUsed: "3Gi"    # Đã xài 3GB / Limit: 8GB
    storage:
      used: "12Gi"  # Tổng ephemeral-storage đang dùng / Limit: 30Gi
    objects:
      podsUsed: 2          # Số lượng Pods thực tế trên K8s mẹ / Limit: 15
      servicesUsed: 1      # Số lượng Services đã tạo / Limit: 5
  
  # Chuẩn báo cáo trạng thái chi tiết của Kubernetes (metav1.Condition)
  conditions:
    - type: "NamespaceReady"
      status: "True"
      lastTransitionTime: "2026-08-02T19:20:00Z"
      reason: "NamespaceCreated"
      message: "Namespace sample-clusterlab-x7z9a created successfully"
    
    - type: "QuotaReady"
      status: "True"
      lastTransitionTime: "2026-08-02T19:20:02Z"
      reason: "ResourceQuotaApplied"
      message: "ResourceQuota and limits enforced successfully"
      
    - type: "NetworkPolicyReady"
      status: "True"
      lastTransitionTime: "2026-08-02T19:20:03Z"
      reason: "PolicyEnforced"
      message: "Network policy rules applied for type: external"
      
    - type: "Ready"
      status: "True"
      lastTransitionTime: "2026-08-02T19:20:10Z"
      reason: "ClusterReady"
      message: "ClusterLab is fully provisioned and accepting InstanceLabs"
``` 
## 1.2. InstanceLab (Máy ảo trong cụm DevOps Labs)
- Định nghĩa Custom Resource Definition (CRD):
```yaml
apiVersion: lab.devops.toiyeuptit.com/v1alpha1
kind: InstanceLab
metadata:
  name: sample-instancelab-master
  labels:
    student_id: "student-01"
    session_id: "session-01"
    lab_id: "lab-01"
spec:
  # Liên kết máy ảo này vào ClusterLab nào (Operator sẽ tự động đẩy Pod vào Namespace tương ứng)
  clusterLabRef: "sample-clusterlab"
  
  # Cấu hình Máy ảo
  image: "sysbox-focal-docker:latest"
  hostname: "controlplane"             # Tùy chỉnh hostname bên trong máy ảo
  imagePullSecrets:
    - name: "regcred"                  # Secret để pull image từ registry private (nếu cần)
  
  # TÀI NGUYÊN TÍNH TOÁN & DISK QUOTA
  resources:
    cpu:
      request: "500m"
      limit: "1"
    memory:
      request: "1Gi"
      limit: "2Gi"
    storage:
      limit: "20Gi"           # Giới hạn tổng disk container (writable layer + logs + /var/lib/docker)
                              # Map sang Pod resources.limits.ephemeral-storage

  # CẤU HÌNH MẠNG & PORT
  ports:
    - name: web-app
      port: 8000
      targetPort: 80
      # Nếu expose = true, Operator tự động tạo Ingress rule HTTPS WSS
      expose: true 
    
    - name: ssh
      port: 22
      targetPort: 22
      # Nếu expose = false, cổng này chỉ có thể được gọi nội bộ bởi các InstanceLab khác trong cùng ClusterLab
      expose: false
```
- Status (Trạng thái của máy ảo trả ngược về để Trang web/Học viên kết nối làm lab ngay):
```yaml
status:
  # Trạng thái máy ảo: Pending | Creating | Running | Stopped | Failed
  phase: "Running"
  
  # Tên Pod thực tế dưới Kubernetes mẹ và IP nội bộ bên trong ClusterLab Namespace
  podName: "sample-instancelab-master-sysbox"
  podIP: "10.42.1.88"
  
  # DANH SÁCH ĐIỂM TRUY CẬP (Endpoints / URLs) DÀNH CHO SINH VIÊN:
  # Operator tự động tạo Ingress/Service rồi gom đường link về đây cho UI/Trang web hiển thị nút bấm!
  accessEndpoints:
    - name: "web-app"
      protocol: "HTTPS"
      url: "https://sample-instancelab-master.sample-clusterlab.lab.toiyeuptit.com"
      internalAddress: "http://sample-instancelab-master-svc.sample-clusterlab-x7z9a.svc.cluster.local:8000"
    - name: "ssh"
      protocol: "TCP"
      internalAddress: "http://sample-instancelab-master-svc.sample-clusterlab-x7z9a.svc.cluster.local:22"
  
  # Danh sách ổ đĩa PVC đã được khởi tạo và bind thành công
  # (Sẽ bổ sung trong tương lai nếu cần expose trạng thái PVC)

  # Chuẩn điều kiện Kubernetes (metav1.Condition)
  conditions:
    - type: "PodReady"
      status: "True"
      lastTransitionTime: "2026-08-02T19:22:15Z"
      reason: "ContainerRunning"
      message: "Sysbox VM instance is running and responsive"
      
    - type: "Ready"
      status: "True"
      lastTransitionTime: "2026-08-02T19:22:16Z"
      reason: "InstanceReady"
      message: "InstanceLab is ready for student connectivity"
```

# 2. Controller Architecture
## 2.1. ClusterLab Controller (`internal/controller/clusterlab_controller.go`)

Controller này đóng vai trò là **"Người quản lý cơ sở hạ tầng"**, chịu trách nhiệm khởi tạo vùng không gian độc lập, áp đặt các giới hạn tài nguyên và xử lý thu hồi hạ tầng khi xóa cụm.

### Các hàm kiến tạo tài nguyên & Vòng đời (Resource Provisioning & Lifecycle)

* `Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)`: Hàm vòng lặp trung tâm. Lắng nghe mọi thay đổi (Create/Update/Delete) của đối tượng `ClusterLab` và điều hướng tới các bước xử lý.
* `reconcileTTL(ctx, clusterLab)`:
  * **Nhiệm vụ:** Quản lý thời hạn sống của cụm Lab theo trường `spec.ttl` để tránh sinh viên lạm dụng hoặc bỏ quên máy ảo chạy vĩnh viễn.
  * **Xử lý Logic & Chống trôi Requeue:** Tính toán thời điểm hết hạn `expiresAt = metadata.creationTimestamp + spec.ttl` và cập nhật vào `status.expiresAt`. Nếu thời gian hiện tại `>= expiresAt`, thi hành ngay lệnh `r.Delete(ctx, clusterLab)`. Nếu chưa tới, luôn luôn trả về `ctrl.Result{RequeueAfter: time.Until(expiresAt)}` tại điểm kết thúc hàm `Reconcile` để đảm bảo bộ đếm ngược không bị trôi dù có bất kỳ sự kiện chèn giữa nào.
* `reconcileFinalizer(ctx, clusterLab)` *(Quan trọng - Tối ưu hóa Cascade Deletion)*:
  * **Nhiệm vụ:** Xử lý tiêu huỷ hạ tầng và giải phóng tài nguyên một cách tối ưu nhờ cơ chế Garbage Collector của Kubernetes.
  * **Quy trình Finalizer Tối ưu (4 bước vàng):**
    1. Bắt tín hiệu tiêu huỷ `DeletionTimestamp != nil`.
    2. **Gửi lệnh `Delete` trực tiếp tới K8s Namespace:** K8s Garbage Collector sẽ tự động loại bỏ toàn bộ tài nguyên trong Namespace đó.
    3. **Đợi tháo gỡ hoàn toàn:** Trả về `Requeue: true` kiên nhẫn đợi cho đến khi API Kubernetes trả về lỗi `apierrors.IsNotFound(err)` khi tìm Namespace — minh chứng hợp lệ rằng Namespace và trọn vẹn hạ tầng lab đã biến mất 100%.
    4. **Hoàn tất:** Gỡ thẻ Finalizer khỏi ClusterLab để hoàn tất quy trình xóa bản ghi CR.
* `reconcileNamespace(ctx, clusterLab, nsName)`:
  * **Nhiệm vụ:** Kiểm tra xem K8s Namespace tương ứng đã tồn tại chưa. Nếu chưa, tạo mới với nhãn quản lý và chuyển tiếp các label `student_*`, `lab_*`, `session_*`.
* `reconcileResourceQuota(ctx, clusterLab, nsName)`:
  * **Nhiệm vụ:** Đọc trường `spec.quota` và dịch sang chuẩn `corev1.ResourceQuota` của Kubernetes bên trong Namespace.
* `reconcileNetworkPolicy(ctx, clusterLab, nsName)`:
  * **Nhiệm vụ:** Dịch trường `spec.network.type` sang `networkingv1.NetworkPolicy` (isolate / internal / external).

### Cơ chế Theo Dõi Thời Gian Thực (SetupWithManager)
Thay vì chỉ lắng nghe sự thay đổi của một mình `ClusterLab`, Controller theo dõi sát sao sự ra đời/biến động của các máy ảo con:
```go
func (r *ClusterLabReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&labv1alpha1.ClusterLab{}).
        // Lắng nghe sự biến động của InstanceLab để tức thời tính lại InstanceCount & QuotaUsage cho ClusterLab cha!
        Watches(&labv1alpha1.InstanceLab{}, handler.EnqueueRequestsFromMapFunc(r.findParentClusterLab)).
        Named("clusterlab").
        Complete(r)
}
```

---

## 2.2. InstanceLab Controller (`internal/controller/instancelab_controller.go`)

Controller này đóng vai trò là **"Người vận hành máy ảo"**, chịu trách nhiệm lắp ráp Pod (Sysbox runtime), cấu hình dịch vụ mạng và tổng hợp danh sách URL bảo mật.

### Các hàm kiến tạo tài nguyên & Xác thực (Resource Provisioning)

* `Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)`: Vòng lặp trung tâm của InstanceLab.
* `resolveParentClusterLab(ctx, instanceLab)` *(Bước xác thực tiên quyết & Gán OwnerReference)*:
  * **Nhiệm vụ:** Đọc trường `spec.clusterLabRef`, tìm `ClusterLab` cha trong cùng Namespace mẹ để kiểm tra xem đã ở phase `Running` hay chưa.
  * **Xử lý Logic:** Nếu cụm cha chưa sẵn sàng -> Báo `Phase: Pending / WaitingForCluster`. Nếu cụm cha đã sẵn sàng -> Thiết lập **`OwnerReference` của InstanceLab trỏ về ClusterLab** (giúp Cascade delete các CR) và lấy chuỗi `status.targetNamespace` làm địa bàn triển khai workload bên dưới.
* `reconcileSysboxPod(ctx, instanceLab, targetNamespace, pvcName)`:
  * **Nhiệm vụ:** Tạo `corev1.Pod` với `runtimeClassName: sysbox-runc`. Tự động mount PVC vào thư mục `/workspace` để cấp phát vùng lưu trữ an toàn, độc lập cho sinh viên.
  * **Trick Bảo mật Host:** Sử dụng `Lifecycle.PostStart` để tiêm các lệnh che giấu thông tin cấu hình máy chủ vật lý (ẩn `/sys/block` tránh `lsblk` và tạo wrapper cho `df`).
* `reconcilePVC(ctx, instanceLab, targetNamespace)`:
  * **Nhiệm vụ:** Đọc trường `spec.resources.storage.limit` và tự động sinh ra `PersistentVolumeClaim` (PVC) sử dụng StorageClass `openebs-lvm`.
  * **Lợi ích:** Đảm bảo mỗi máy ảo có một ổ đĩa thật (LVM Logical Volume) được cách ly dung lượng tuyệt đối, vượt qua giới hạn của OverlayFS.
* `reconcileFinalizer(ctx, instanceLab)`:
  * **Nhiệm vụ:** Cấu hình Garbage Collector. Khi `InstanceLab` bị xóa, nó sẽ bắt tín hiệu và lập tức xóa PVC tương ứng để OpenEBS thu hồi dung lượng LVM, chống thất thoát tài nguyên lưu trữ (Storage Leak).
* `reconcileServices(ctx, instanceLab, targetNamespace)`:
  * **Nhiệm vụ:** Duyệt mảng `spec.ports` và tạo `corev1.Service` (`ClusterIP`).
* `reconcileIngress(ctx, instanceLab, clusterLab, targetNamespace, svc)`:
  * **Nhiệm vụ:** Lọc ra các port có cờ `expose: true`.
  * **Xử lý Logic:** Sinh ra `networkingv1.Ingress` trỏ về Service phía trên, thực hiện tiêm cấu hình chuẩn:
    * Inject IngressClass từ biến môi trường `DEFAULT_INGRESS_CLASS` (Mặc định: nginx).
    * Inject Annotation chứng chỉ số (`cert-manager.io/cluster-issuer: letsencrypt-prod`).
    * Bổ sung block `tls:` trong `spec` để đảm bảo sinh viên truy cập thông qua tiêu chuẩn **HTTPS WSS (WebSocket Secure)** an toàn tuyệt đối.

### Cơ chế Theo Dõi Tự Sửa Lỗi (SetupWithManager & Ownership)
Để đảm bảo tính tự phục hồi (Self-healing), InstanceLab Controller quản lý trọn đời tài nguyên:
```go
func (r *InstanceLabReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&labv1alpha1.InstanceLab{}).
        Named("instancelab").
        Complete(r)
}
```

---

## 2.3. Khối tiện ích dùng chung (Package `internal/utils`)

* `GetIngressClassName() string`: Đọc biến môi trường `DEFAULT_INGRESS_CLASS` (mặc định: `nginx`).
* `GenerateIngressHost(instanceName, clusterName, customDomain string) string`: Sinh tên miền động cho máy ảo.
* `GenerateTargetNamespace(clusterLabName string) string`: Chuẩn hóa độ dài tên Namespace trong ngưỡng 63 ký tự kết hợp băm SHA-1 chống va chạm.

> **Đã loại bỏ:** `GetStorageClassName()` — không còn cần thiết vì không dùng StorageClass nữa.

---

# 3. Những "Cạm bẫy" Kỹ thuật & Nguyên tắc Thực chiến (Gotchas & Recommendations)

### A. Tối ưu hóa logic Finalizer & Cascade Deletion
* **Tận dụng sức mạnh của K8s Garbage Collector & OwnerReference:** Ngay khi `InstanceLab` hình thành, gán lập tức `OwnerReference` trỏ về `ClusterLab` cha. Khi `ClusterLab` bị tiêu huỷ, K8s GC sẽ tự động thanh lọc các bản ghi `InstanceLab`.

### B. Xử lý triệt để hiện tượng "Requeue bị trôi" đối với TTL
* Trong mỗi chu kỳ Reconcile, hãy luôn luôn so sánh `time.Now()` với `expiresAt` và trả về `RequeueAfter` cho phần thời gian còn lại.

### C. Ingress Class và tiêu chuẩn Mạng Bảo Mật (HTTPS / TLS)
* Khi tạo Ingress trong `reconcileIngress`, luôn khai báo rõ ràng `ingressClassName` và tích hợp TLS Cert-Manager để hỗ trợ kết nối WebSocket WSS mượt mà.

### D. Kiểm soát Độ dài Tên tối đa 63 Ký tự (RFC 1123)
* Khi tạo tên động cho Namespace, Service hay Hostname, luôn có bước kiểm soát độ dài và dùng băm SHA-1 nếu vượt quá 63 ký tự.

### E. Quy tắc Ưu tiên Gắn Finalizer (The Finalizer-First Rule)
* Trong hàm `Reconcile` của cả `ClusterLab` và `InstanceLab`, việc thêm và update Finalizer phải là bước xử lý đầu tiên tuyệt đối, TRƯỚC KHI tạo bất kỳ Namespace, ResourceQuota hay Pod nào.

### F. Khai báo Đầy Đủ Chú Giải Quyền Truy Cập (RBAC Markers)
* **Mẫu quy ước RBAC cho ClusterLab Controller:**
  ```go
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=clusterlabs,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=clusterlabs/status,verbs=get;update;patch
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=clusterlabs/finalizers,verbs=update
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=instancelabs,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=core,resources=namespaces;resourcequotas,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
  ```
* **Mẫu quy ước RBAC cho InstanceLab Controller:**
  ```go
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=instancelabs,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=instancelabs/status,verbs=get;update;patch
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=instancelabs/finalizers,verbs=update
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=clusterlabs,verbs=get;list;watch
  // +kubebuilder:rbac:groups=core,resources=pods;services,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
  ```

---

# 4. Chi tiết các Tài nguyên Bản địa (Native K8s Manifests) được Operator tạo ra

Khi Backend hoặc người quản trị khởi tạo các đối tượng CRD (`ClusterLab` và `InstanceLab`), **TYP-Operator** sẽ tự động dịch chuyển và quản trị các tài nguyên chuẩn mực dưới tầng Kubernetes:

## 4.1. Nhóm Tài nguyên cấp Cụm (Sinh ra từ `ClusterLab`)

Khi một `ClusterLab` được khởi tạo, Operator sẽ quy hoạch một vùng cách ly hoàn chỉnh trong hệ thống Kubernetes, bao gồm:

### 4.1.1. Namespace (Vùng cách ly phòng lab)
Mọi đối tượng thuộc cụm sẽ được giam giữ gọn gàng trong một Namespace riêng biệt mang nhãn từ đơn đăng ký Lab/Session/Student:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: lab-clusterlab-sample              # Mã định danh Namespace động (SHA-1 nếu vượt 63 ký tự)
  labels:
    app.kubernetes.io/managed-by: typ-operator
    lab.devops.toiyeuptit.com/cluster-lab: clusterlab-sample
    student_id: "student-01"                   # Tự động kế thừa các nhãn student_, lab_, session_ từ Cụm
    session_id: "session-01"
    lab_id: "lab-01"
```

### 4.1.2. ResourceQuota (Hàng rào Ngân sách Tài nguyên & Lưu trữ)
Để đảm bảo một cụm Lab không tiêu hao vượt mức tài nguyên chung, Operator dịch thông số `spec.quota` thành một `ResourceQuota` áp đặt lên Namespace:
```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: lab-resource-quota
  namespace: lab-clusterlab-sample
spec:
  hard:
    # Quota Tính toán
    limits.cpu: "4"
    requests.cpu: "4"
    limits.memory: "8Gi"
    requests.memory: "8Gi"
    
    # Quota Số lượng Đối tượng
    pods: "15"
    services: "5"
    
    # Quota Ephemeral Storage (emptyDir trên disk node) — không cần StorageClass
    limits.ephemeral-storage: "30Gi"
```

### 4.1.3. NetworkPolicy (Tường lửa Giao thoa Mạng)
Căn cứ vào trường `spec.network.type`, Operator sinh ra một bản tường lửa cho toàn bộ các Pod/máy ảo bên trong Namespace:

#### 1. Chế độ `isolate` (Cô lập tuyệt đối - Cấm mọi luồng Ingress):
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: lab-network-policy
  namespace: lab-clusterlab-sample
spec:
  podSelector: {}                       # Áp dụng lên mọi máy ảo trong Namespace
  policyTypes:
    - Ingress
  ingress: []                           # Mảng rỗng = Chặn đứng mọi lưu lượng đến
```

#### 2. Chế độ `internal` (Mạng nội bộ giữa các máy ảo trong phòng Lab):
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: lab-network-policy
  namespace: lab-clusterlab-sample
spec:
  podSelector: {}
  policyTypes:
    - Ingress
  ingress:
    - from:
        - podSelector: {}               # Chỉ cho phép luồng từ các Pod cùng Namespace
```

#### 3. Chế độ `external` (Mở kết nối cho phép truy cập từ Ingress / Web Terminal):
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: lab-network-policy
  namespace: lab-clusterlab-sample
spec:
  podSelector: {}
  policyTypes:
    - Ingress
  ingress:
    - from: []                          # Cho phép luồng từ bộ định tuyến ngoại tuyến (Ingress Controller)
```

---

## 4.2. Nhóm Tài nguyên cấp Máy ảo (Sinh ra từ `InstanceLab`)

Khi khởi tạo một đối tượng `InstanceLab` trong cụm Lab có sẵn, Operator sẽ tự động lắp ráp bộ tứ tài nguyên workload dưới đây:

### 4.2.1. Storage (OpenEBS LVM & Ephemeral)
Sử dụng công nghệ **OpenEBS LVM LocalPV** kết hợp Ansible tự động tạo Loopback Device (50GB). Quá trình này giúp chia nhỏ ổ đĩa ảo bằng LVM mà không làm ảnh hưởng đến đĩa vật lý của máy chủ.

* **`/workspace` (Persistent/LVM):** Operator sẽ tự động tạo một `PersistentVolumeClaim` sử dụng StorageClass `openebs-lvm` với dung lượng đúng bằng `spec.resources.storage.limit`. Ổ đĩa này được mount vào `/workspace` bên trong container.
* Khi `InstanceLab` bị xóa, logic Finalizer của Operator sẽ tự động gọi lệnh xóa PVC, hoàn trả lại dung lượng về Pool LVM ảo của cụm.

### 4.2.2. Pod (Máy ảo Docker-in-Docker chạy Sysbox Runtime)
Cấu hình Pod được Operator lắp ráp với đầy đủ các thuộc tính bảo mật Sysbox, cô lập User-Namespace và chia sẻ thông số từ `lxcfs`:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: instancelab-sample-sysbox
  namespace: lab-clusterlab-sample
  labels:
    app: instancelab-sample-sysbox
    app.kubernetes.io/managed-by: typ-operator
    lab.devops.toiyeuptit.com/cluster-lab: clusterlab-sample
    lab.devops.toiyeuptit.com/instance-lab: instancelab-sample
spec:
  enableServiceLinks: false                  # Tắt tự động mount env biến môi trường K8s
  hostUsers: false                           # Bảo mật User-Namespace của Sysbox
  hostname: controlplane                     # Tùy chỉnh hostname theo trường spec.hostname
  runtimeClassName: sysbox-runc              # Runtime engine của Sysbox
  restartPolicy: Never                       # Vòng đời không tự khởi tạo vô định
  nodeSelector:
    sysbox-install: "yes"                    # Bắt buộc lên kế hoạch trên node có cài sysbox
  imagePullSecrets:
    - name: regcred                          # Hỗ trợ pull image từ private registry
  containers:
    - name: main                             # Container gốc của máy ảo
      image: ubuntu:24.04
      imagePullPolicy: IfNotPresent
      command:
        - /sbin/init                         # Chạy systemd chuẩn cho môi trường Sysbox
      ports:
        - name: terminal
          containerPort: 7681
          protocol: TCP
      resources:
        requests:
          cpu: "500m"          # ← spec.resources.cpu.request
          memory: "512Mi"      # ← spec.resources.memory.request
        limits:
          cpu: "1"             # ← spec.resources.cpu.limit
          memory: "1Gi"        # ← spec.resources.memory.limit
          ephemeral-storage: "20Gi"  # Giới hạn phân vùng root (OverlayFS)
      lifecycle:
        postStart:
          exec:
            # TRICK BẢO MẬT: Ẩn thông tin cấu hình phần cứng vật lý của máy chủ khỏi sinh viên!
            # 1. Che df: Dùng wrapper script ẩn các phân vùng host (overlay) và RAM ảo (tmpfs).
            # 2. Che lsblk/fdisk: Đè tmpfs trống lên /sys/block và /sys/class/block.
            command: 
              - /bin/bash
              - -c
              - "echo -e '#!/bin/bash\\n/usr/bin/df \"$@\" | grep -vE \"^overlay|^tmpfs|^shm|^/dev/nvme|^/dev/sda|^/dev/vda\"' > /usr/local/bin/df && chmod +x /usr/local/bin/df && mount -t tmpfs tmpfs /sys/block && mount -t tmpfs tmpfs /sys/class/block"
      volumeMounts:
        # Mount ổ đĩa LVM OpenEBS an toàn cho workspace sinh viên
        - name: workspace-volume
          mountPath: /workspace
        # Mount toàn bộ các điểm báo cáo thông lượng LXCFS từ Host vào /proc
        - name: lxcfs-proc-cpuinfo
          mountPath: /proc/cpuinfo
        - name: lxcfs-proc-meminfo
          mountPath: /proc/meminfo
        - name: lxcfs-proc-diskstats
          mountPath: /proc/diskstats
        - name: lxcfs-proc-swaps
          mountPath: /proc/swaps
        - name: lxcfs-proc-uptime
          mountPath: /proc/uptime
  volumes:
    # Ổ đĩa LVM động cấp phát bởi OpenEBS
    - name: workspace-volume
      persistentVolumeClaim:
        claimName: instancelab-sample-workspace
    - name: lxcfs-proc-cpuinfo
      hostPath:
        path: /var/lib/lxcfs/proc/cpuinfo
        type: File
    - name: lxcfs-proc-meminfo
      hostPath:
        path: /var/lib/lxcfs/proc/meminfo
        type: File
    - name: lxcfs-proc-diskstats
      hostPath:
        path: /var/lib/lxcfs/proc/diskstats
        type: File
    - name: lxcfs-proc-swaps
      hostPath:
        path: /var/lib/lxcfs/proc/swaps
        type: File
    - name: lxcfs-proc-uptime
      hostPath:
        path: /var/lib/lxcfs/proc/uptime
        type: File
```

### 4.2.3. Service (Điểm kết nối mạng nội bộ ClusterIP)
Tạo ra IP tĩnh và định danh DNS ổn định, phục vụ cho lưu lượng trong cụm Lab và làm Backend bắt buộc cho Ingress:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: instancelab-sample-svc
  namespace: lab-clusterlab-sample
  labels:
    app.kubernetes.io/managed-by: typ-operator
    lab.devops.toiyeuptit.com/cluster-lab: clusterlab-sample
    lab.devops.toiyeuptit.com/instance-lab: instancelab-sample
spec:
  type: ClusterIP
  selector:
    app: instancelab-sample-sysbox         # Liên kết khớp tuyệt đối với Pod
  ports:
    - name: terminal
      port: 8080
      targetPort: 7681
      protocol: TCP
```

### 4.2.4. Ingress (Cổng kết nối SSL HTTPS ngoài cụm)
Chỉ khởi tạo khi có ít nhất 1 Port đặt `expose: true`. Tích hợp Cert-Manager để tự động xin chứng chỉ số Let's Encrypt và hỗ trợ kết nối WebSocket WSS cho Terminal:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: instancelab-sample-ingress
  namespace: lab-clusterlab-sample
  labels:
    app.kubernetes.io/managed-by: typ-operator
    lab.devops.toiyeuptit.com/cluster-lab: clusterlab-sample
    lab.devops.toiyeuptit.com/instance-lab: instancelab-sample
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod             # Xin chứng chỉ Let's Encrypt
    nginx.ingress.kubernetes.io/ssl-redirect: "true"             # Bắt buộc chuyển sang HTTPS
    nginx.ingress.kubernetes.io/proxy-body-size: "1000m"         # Cho phép upload tệp lớn lên máy ảo
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"       # Giữ kết nối Web Terminal/WebSocket lên tới 1 giờ
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        # Tự động kiến tạo tên miền định danh cho sinh viên
        - instancelab-sample.clusterlab-sample.devops.toiyeuptit.com
      secretName: instancelab-sample-tls-secret
  rules:
    - host: instancelab-sample.clusterlab-sample.devops.toiyeuptit.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: instancelab-sample-svc                 # Trỏ thẳng về Service
                port:
                  number: 8080
```