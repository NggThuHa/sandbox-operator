# 1. API Architecture
## 1.1. VirtualCluster (Cụm máy ảo DevOps Lab)
- Định nghĩa Custom Resource Definition (CRD):
```yaml
apiVersion: lab.devops.toiyeuptit.com/v1alpha1
kind: VirtualCluster
metadata:
  name: sample-virtualcluster
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
    # internal: Các VirtualInstance gọi được nhau nội bộ trong Namespace, chặn ngoài
    # external: Mở Ingress/LB cho phép bên ngoài truy cập vào
    type: "external" 

  # Giới hạn tài nguyên (Quota) cho toàn bộ cụm
  quota:
    storage:
      localLimit: "50Gi"    # Giới hạn tổng dung lượng ổ cứng vật lý trên node (local-path)
      networkLimit: "10Gi"  # Giới hạn tổng dung lượng trên hệ thống lưu trữ mạng (longhorn)
    
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

  # Tên K8s Namespace thực tế mà Operator đã khởi tạo cho VirtualCluster này bên dưới (không vượt quá 63 ký tự)
  targetNamespace: "sample-virtualcluster-x7z9a"
  
  # Số lượng máy ảo (VirtualInstance) đang gắn vào cụm này
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
      localUsed: "25Gi"    # Đã xài 25GB / Limit: 50GB
      networkUsed: "5Gi"   # Đã xài 5GB / Limit: 10GB
    objects:
      podsUsed: 2          # Số lượng Pods thực tế trên K8s mẹ / Limit: 15
      servicesUsed: 1      # Số lượng Services đã tạo / Limit: 5
  
  # Chuẩn báo cáo trạng thái chi tiết của Kubernetes (metav1.Condition)
  conditions:
    - type: "NamespaceReady"
      status: "True"
      lastTransitionTime: "2026-08-02T19:20:00Z"
      reason: "NamespaceCreated"
      message: "Namespace sample-virtualcluster-x7z9a created successfully"
    
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
      message: "VirtualCluster is fully provisioned and accepting VirtualInstances"
``` 
## 1.2. VirtualInstance (Máy ảo trong cụm DevOps Labs)
- Định nghĩa Custom Resource Definition (CRD):
```yaml
apiVersion: lab.devops.toiyeuptit.com/v1alpha1
kind: VirtualInstance
metadata:
  name: sample-virtualinstance-master
  labels:
    student_id: "student-01"
    session_id: "session-01"
    lab_id: "lab-01"
spec:
  # Liên kết máy ảo này vào VirtualCluster nào (Operator sẽ tự động đẩy Pod vào Namespace tương ứng)
  virtualClusterRef: "sample-virtualcluster"
  
  # Cấu hình Container Runtime
  image: "sysbox-focal-docker:latest"
  runtimeClassName: "sysbox-runc"
  
  # TÀI NGUYÊN TÍNH TOÁN
  resources:
    cpu:
      request: "500m"
      limit: "1"
    memory:
      request: "1Gi"
      limit: "2Gi"

  storage:
    # Ổ đĩa root (OS) mặc định dùng local để boot nhanh và I/O cao cho Sysbox Docker engine
    root:
      size: "10Gi"
      type: "local" 
    
    # Khai báo các volume gắn thêm
    dataVolumes:
      # 1. LOCAL STORAGE: Dành cho những thư mục cần I/O cực cao, không cần lưu trữ phân tán lâu dài
      - name: docker-cache
        mountPath: "/var/lib/docker"
        size: "20Gi"
        type: "local"
      
      # 2. NETWORK STORAGE: Dành cho thư mục chứa code, bài làm của sinh viên cần giữ lại bền bỉ
      - name: student-workspace
        mountPath: "/home/ubuntu/workspace"
        size: "5Gi"
        type: "network"

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
      # Nếu expose = false, cổng này chỉ có thể được gọi nội bộ bởi các VirtualInstance khác trong cùng VirtualCluster
      expose: false
```
- Status (Trạng thái của máy ảo trả ngược về để Trang web/Học viên kết nối làm lab ngay):
```yaml
status:
  # Trạng thái máy ảo: Pending | Creating | Running | Stopped | Failed
  phase: "Running"
  
  # Tên Pod thực tế dưới Kubernetes mẹ và IP nội bộ bên trong VirtualCluster Namespace
  podName: "sample-virtualinstance-master-sysbox"
  podIP: "10.42.1.88"
  
  # DANH SÁCH ĐIỂM TRUY CẬP (Endpoints / URLs) DÀNH CHO SINH VIÊN:
  # Operator tự động tạo Ingress/Service rồi gom đường link về đây cho UI/Trang web hiển thị nút bấm!
  accessEndpoints:
    - name: "web-app"
      protocol: "HTTPS"
      url: "https://sample-virtualinstance-master.sample-virtualcluster.lab.toiyeuptit.com"
      internalAddress: "http://sample-virtualinstance-master-svc.sample-virtualcluster-x7z9a.svc.cluster.local:8000"
    - name: "ssh"
      protocol: "TCP"
      internalAddress: "http://sample-virtualinstance-master-svc.sample-virtualcluster-x7z9a.svc.cluster.local:22"
  
  # Trạng thái của các ổ đĩa (PVCs) đã đính kèm vào máy ảo
  volumesStatus:
    rootVolume:
      pvcName: "sample-virtualinstance-master-root-pvc"
      status: "Bound"
    dataVolumes:
      - name: "docker-cache"
        pvcName: "sample-virtualinstance-master-docker-cache-pvc"
        status: "Bound"
      - name: "student-workspace"
        pvcName: "sample-virtualinstance-master-student-workspace-pvc"
        status: "Bound"

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
      message: "VirtualInstance is ready for student connectivity"
```

# 2. Controller Architecture
## 2.1. VirtualCluster Controller (`internal/controller/virtualcluster_controller.go`)

Controller này đóng vai trò là **"Người quản lý cơ sở hạ tầng"**, chịu trách nhiệm khởi tạo vùng không gian độc lập, áp đặt các giới hạn tài nguyên và xử lý thu hồi hạ tầng khi xóa cụm.

### Các hàm kiến tạo tài nguyên & Vòng đời (Resource Provisioning & Lifecycle)

* `Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)`: Hàm vòng lặp trung tâm. Lắng nghe mọi thay đổi (Create/Update/Delete) của đối tượng `VirtualCluster` và điều hướng tới các bước xử lý.
* `reconcileTTL(ctx, virtualCluster)`:
  * **Nhiệm vụ:** Quản lý thời hạn sống của cụm Lab theo trường `spec.ttl` để tránh sinh viên lạm dụng hoặc bỏ quên máy ảo chạy vĩnh viễn.
  * **Xử lý Logic & Chống trôi Requeue:** Tính toán thời điểm hết hạn `expiresAt = metadata.creationTimestamp + spec.ttl` và cập nhật vào `status.expiresAt`. Nếu thời gian hiện tại `>= expiresAt`, thi hành ngay lệnh `r.Delete(ctx, virtualCluster)`. Nếu chưa tới, luôn luôn trả về `ctrl.Result{RequeueAfter: time.Until(expiresAt)}` tại điểm kết thúc hàm `Reconcile` để đảm bảo bộ đếm ngược không bị trôi dù có bất kỳ sự kiện chèn giữa nào.
* `reconcileFinalizer(ctx, virtualCluster)` *(Quan trọng - Tối ưu hóa Cascade Deletion)*:
  * **Nhiệm vụ:** Xử lý tiêu huỷ hạ tầng và giải phóng tài nguyên một cách tối ưu nhờ cơ chế Garbage Collector của Kubernetes.
  * **Quy trình Finalizer Tối ưu (4 bước vàng):**
    1. Bắt tín hiệu tiêu huỷ `DeletionTimestamp != nil`.
    2. **Gửi lệnh `Delete` trực tiếp tới K8s Namespace:** K8s Garbage Collector sẽ tự động loại bỏ toàn bộ tài nguyên trong Namespace đó.
    3. **Đợi tháo gỡ hoàn toàn:** Trả về `Requeue: true` kiên nhẫn đợi cho đến khi API Kubernetes trả về lỗi `apierrors.IsNotFound(err)` khi tìm Namespace — minh chứng hợp lệ rằng Namespace và trọn vẹn hạ tầng lab đã biến mất 100%.
    4. **Hoàn tất:** Gỡ thẻ Finalizer khỏi VirtualCluster để hoàn tất quy trình xóa bản ghi CR.
* `reconcileNamespace(ctx, virtualCluster, nsName)`:
  * **Nhiệm vụ:** Kiểm tra xem K8s Namespace tương ứng đã tồn tại chưa. Nếu chưa, tạo mới với nhãn quản lý và chuyển tiếp các label `student_*`, `lab_*`, `session_*`.
* `reconcileResourceQuota(ctx, virtualCluster, nsName)`:
  * **Nhiệm vụ:** Đọc trường `spec.quota` và dịch sang chuẩn `corev1.ResourceQuota` của Kubernetes bên trong Namespace.
* `reconcileNetworkPolicy(ctx, virtualCluster, nsName)`:
  * **Nhiệm vụ:** Dịch trường `spec.network.type` sang `networkingv1.NetworkPolicy` (isolate / internal / external).

### Cơ chế Theo Dõi Thời Gian Thực (SetupWithManager)
Thay vì chỉ lắng nghe sự thay đổi của một mình `VirtualCluster`, Controller theo dõi sát sao sự ra đời/biến động của các máy ảo con:
```go
func (r *VirtualClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&labv1alpha1.VirtualCluster{}).
        // Lắng nghe sự biến động của VirtualInstance để tức thời tính lại InstanceCount & QuotaUsage cho VirtualCluster cha!
        Watches(&labv1alpha1.VirtualInstance{}, handler.EnqueueRequestsFromMapFunc(r.findParentVirtualCluster)).
        Named("virtualcluster").
        Complete(r)
}
```

---

## 2.2. VirtualInstance Controller (`internal/controller/virtualinstance_controller.go`)

Controller này đóng vai trò là **"Người vận hành máy ảo"**, chịu trách nhiệm lắp ráp Pod (Sysbox runtime), gắn ổ cứng, cấu hình dịch vụ mạng và tổng hợp danh sách URL bảo mật.

### Các hàm kiến tạo tài nguyên & Xác thực (Resource Provisioning)

* `Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)`: Vòng lặp trung tâm của VirtualInstance.
* `resolveParentVirtualCluster(ctx, virtualInstance)` *(Bước xác thực tiên quyết & Gán OwnerReference)*:
  * **Nhiệm vụ:** Đọc trường `spec.virtualClusterRef`, tìm `VirtualCluster` cha trong cùng Namespace mẹ để kiểm tra xem đã ở phase `Running` hay chưa.
  * **Xử lý Logic:** Nếu cụm cha chưa sẵn sàng -> Báo `Phase: Pending / WaitingForCluster`. Nếu cụm cha đã sẵn sàng -> Thiết lập **`OwnerReference` của VirtualInstance trỏ về VirtualCluster** (giúp Cascade delete các CR) và lấy chuỗi `status.targetNamespace` làm địa bàn triển khai workload bên dưới.
* `reconcilePVCs(ctx, virtualInstance, targetNamespace)`:
  * **Nhiệm vụ:** Duyệt qua mảng `spec.storage.dataVolumes` (và cả root volume). Khảo sát cấu hình từ `utils.GetStorageClassName()` và khởi tạo các `PersistentVolumeClaim`.
* `reconcileSysboxPod(ctx, virtualInstance, targetNamespace, pvcMap)`:
  * **Nhiệm vụ:** Tạo `corev1.Pod` mang bóng dáng của một máy ảo Sysbox Docker-in-Docker với `runtimeClassName: sysbox-runc`.
* `reconcileServices(ctx, virtualInstance, targetNamespace)`:
  * **Nhiệm vụ:** Duyệt mảng `spec.ports` và tạo `corev1.Service` (`ClusterIP`).
* `reconcileIngress(ctx, virtualInstance, virtualCluster, targetNamespace, svc)`:
  * **Nhiệm vụ:** Lọc ra các port có cờ `expose: true`.
  * **Xử lý Logic:** Sinh ra `networkingv1.Ingress` trỏ về Service phía trên, thực hiện tiêm cấu hình chuẩn:
    * Inject IngressClass từ biến môi trường `DEFAULT_INGRESS_CLASS` (Mặc định: nginx).
    * Inject Annotation chứng chỉ số (`cert-manager.io/cluster-issuer: letsencrypt-prod`).
    * Bổ sung block `tls:` trong `spec` để đảm bảo sinh viên truy cập thông qua tiêu chuẩn **HTTPS WSS (WebSocket Secure)** an toàn tuyệt đối.

### Cơ chế Theo Dõi Tự Sửa Lỗi (SetupWithManager & Ownership)
Để đảm bảo tính tự phục hồi (Self-healing), VirtualInstance Controller quản lý trọn đời tài nguyên:
```go
func (r *VirtualInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&labv1alpha1.VirtualInstance{}).
        Named("virtualinstance").
        Complete(r)
}
```

---

## 2.3. Khối tiện ích dùng chung (Package `internal/utils`)

* `GetStorageClassName(volumeType string) string`: Trả về StorageClass thích hợp từ biến môi trường (`DEFAULT_LOCAL_STORAGE_CLASS` / `DEFAULT_NETWORK_STORAGE_CLASS`).
* `GetIngressClassName() string`: Đọc biến môi trường `DEFAULT_INGRESS_CLASS` (mặc định: `nginx`).
* `GenerateIngressHost(instanceName, clusterName, customDomain string) string`: Sinh tên miền động cho máy ảo.
* `GenerateTargetNamespace(virtualClusterName string) string`: Chuẩn hóa độ dài tên Namespace trong ngưỡng 63 ký tự kết hợp băm SHA-1 chống va chạm.

---

# 3. Những "Cạm bẫy" Kỹ thuật & Nguyên tắc Thực chiến (Gotchas & Recommendations)

### A. Tối ưu hóa logic Finalizer & Cascade Deletion
* **Tận dụng sức mạnh của K8s Garbage Collector & OwnerReference:** Ngay khi `VirtualInstance` hình thành, gán lập tức `OwnerReference` trỏ về `VirtualCluster` cha. Khi `VirtualCluster` bị tiêu huỷ, K8s GC sẽ tự động thanh lọc các bản ghi `VirtualInstance`.

### B. Xử lý triệt để hiện tượng "Requeue bị trôi" đối với TTL
* Trong mỗi chu kỳ Reconcile, hãy luôn luôn so sánh `time.Now()` với `expiresAt` và trả về `RequeueAfter` cho phần thời gian còn lại.

### C. Ingress Class và tiêu chuẩn Mạng Bảo Mật (HTTPS / TLS)
* Khi tạo Ingress trong `reconcileIngress`, luôn khai báo rõ ràng `ingressClassName` và tích hợp TLS Cert-Manager để hỗ trợ kết nối WebSocket WSS mượt mà.

### D. Kiểm soát Độ dài Tên tối đa 63 Ký tự (RFC 1123)
* Khi tạo tên động cho Namespace, Service hay Hostname, luôn có bước kiểm soát độ dài và dùng băm SHA-1 nếu vượt quá 63 ký tự.

### E. Quy tắc Ưu tiên Gắn Finalizer (The Finalizer-First Rule)
* Trong hàm `Reconcile` của cả `VirtualCluster` và `VirtualInstance`, việc thêm và update Finalizer phải là bước xử lý đầu tiên tuyệt đối, TRƯỚC KHI tạo bất kỳ Namespace, ResourceQuota hay Pod nào.

### F. Khai báo Đầy Đủ Chú Giải Quyền Truy Cập (RBAC Markers)
* **Mẫu quy ước RBAC cho VirtualCluster Controller:**
  ```go
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualclusters,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualclusters/status,verbs=get;update;patch
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualclusters/finalizers,verbs=update
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualinstances,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=core,resources=namespaces;resourcequotas,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
  ```
* **Mẫu quy ước RBAC cho VirtualInstance Controller:**
  ```go
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualinstances,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualinstances/status,verbs=get;update;patch
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualinstances/finalizers,verbs=update
  // +kubebuilder:rbac:groups=lab.devops.toiyeuptit.com,resources=virtualclusters,verbs=get;list;watch
  // +kubebuilder:rbac:groups=core,resources=pods;services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
  // +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
  ```